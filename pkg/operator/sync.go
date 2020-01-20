package operator

import (
	"fmt"
	"sort"
	"strings"
	"time"

	aemv1beta1 "github.com/xumak-grid/aem-operator/pkg/apis/aem/v1beta1"
	"github.com/xumak-grid/aem-operator/pkg/k8s"
	aemconfig "github.com/xumak-grid/go-grid/pkg/aem/config"

	"github.com/xumak-grid/go-grid/pkg/pgen"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	podInitializedAnnotation = "initialized"
)

var passwordGenerator = pgen.NewGenerator()

type filterFunc func(pod *v1.Pod) bool

func filterPods(runmodes ...string) filterFunc {
	return func(pod *v1.Pod) bool {
		return isInSlice(pod.Labels["runmode"], runmodes)
	}
}

func (ac *AEMDeploymentController) sync(key string) error {
	startTime := time.Now()
	defer func() {
		ac.logger.Infof("Finished syncing statefulset %s (%s)", key, time.Now().Sub(startTime))
	}()

	obj, exists, err := ac.aemInformer.GetIndexer().GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		// TODO: Check orphan pods.
		ac.logger.Info("AEM deployment has been deleted")
		return nil
	}

	deployment := (obj.(*aemv1beta1.AEMDeployment)).DeepCopy()

	if deployment.Status.Phase == aemv1beta1.DeploymentPhaseNone {

		// creating Initial services
		err := k8s.CreateServices(ac.clientSet, deployment)
		if err != nil {
			return err
		}
		ac.logger.Info("Initial services successfully created")

		// creating initial confiMaps
		err = k8s.SetUpConfigMaps(ac.clientSet, deployment)
		if err != nil {
			return err
		}
		ac.logger.Info("Initial configMaps successfully created")

		deployment.Status.Phase = aemv1beta1.DeploymentPhaseCreating
		_, err = ac.aemcli.AemV1beta1().AEMDeployments(deployment.Namespace).Update(deployment)
		if err != nil {
			ac.logger.Error("Error updating status", err)
		}
	}

	name := deployment.Name
	podList, _ := ac.podInformer.
		Lister().
		Pods(deployment.Namespace).
		List(labels.SelectorFromSet(LabelsForDeployment(name)))

	authorPods := GetPods(podList, filterPods("author"))

	// Check if there is an author running and ready.
	totalAuthors := len(authorPods)
	authors := deployment.Spec.Authors.Replicas
	updateStatus := false
	if totalAuthors != authors {
		ac.logger.Info("Unbalanced authors required")
		ac.resizeDeployment(authorPods, authors, deployment, "author")
		updateStatus = totalAuthors > 0
	}
	publishPods := GetPods(podList, filterPods("publish"))
	dispatcherPods := GetPods(podList, filterPods("dispatcher"))

	publishers := deployment.Spec.Publishers.Replicas
	dispatchers := deployment.Spec.Dispatchers.Replicas

	if len(publishPods) != publishers {
		ac.logger.Info("Unbalanced publishers")
		ac.resizeDeployment(publishPods, publishers, deployment, "publish")
		updateStatus = len(publishPods) > 0
	}
	if len(dispatcherPods) != dispatchers {
		ac.logger.Info("Unbalanced dispatchers")
		ac.resizeDeployment(dispatcherPods, dispatchers, deployment, "dispatcher")
		updateStatus = len(dispatcherPods) > 0
	}

	if updateStatus {
		// Consider to add this as a deployment condition insteaad of a phase.
		deployment.Status.Phase = aemv1beta1.DeploymentPhaseResizing
		_, err := ac.aemcli.AemV1beta1().AEMDeployments(deployment.Namespace).Update(deployment)
		if err != nil {
			ac.logger.Error("Error updating status", err)
			return err
		}
	}

	allPods := GetPods(podList, filterPods("author", "publish", "dispatcher"))
	for _, pod := range allPods {
		if !isHealthy(pod) {
			ac.logger.Infof("Deployment %s not ready", deployment.Name)
			return fmt.Errorf("Pods not ready")
		}
	}
	if deployment.Status.Phase != aemv1beta1.DeploymentPhaseRunning {
		deployment.Status.Phase = aemv1beta1.DeploymentPhaseRunning
		_, err := ac.aemcli.AemV1beta1().AEMDeployments(deployment.Namespace).Update(deployment)
		if err != nil {
			ac.logger.Error("Error updating status", err)
			return err
		}
	}
	// Check pod initialization
	for _, pod := range allPods {
		if !isAuthor(pod) && !isPublish(pod) {
			continue
		}
		err := ac.checkPodInitialized(pod.DeepCopy(), deployment)
		if err != nil {
			return err
		}
	}
	// Check pod configuration
	for _, pod := range allPods {
		if isAuthor(pod) {
			err := ac.checkAuthorConfig(pod.DeepCopy(), publishPods, deployment)
			if err != nil {
				ac.logger.Error("Error checking author config", err)
				return err
			}
			continue
		}
		if isPublish(pod) {
			ac.checkPublishConfig(publishPods, deployment)
		}
	}
	return nil
}

func (ac *AEMDeploymentController) getPodPassword(pod *v1.Pod, deployment string) (string, error) {
	podSecretsKey := getPodSecretKey(pod.Namespace, deployment, pod.Name)
	podSecrets, err := ac.secrets.Get(podSecretsKey)
	if err != nil {
		return "", err
	}
	if podSecrets == nil {
		return "", nil
	}
	pwd, ok := podSecrets["password"]
	if !ok || pwd == nil {
		return "", nil
	}
	pwdc, ok := pwd.(string)
	if !ok {
		return "", nil
	}
	return pwdc, nil
}

func parseAgentsList(repAgents []map[string]interface{}, repType string) map[string]interface{} {
	collected := make([]map[string]interface{}, 0)
	for _, a := range repAgents {
		for k, v := range a {
			if k == repType {
				for _, vv := range v.([]interface{}) {
					repAgent, ok := vv.(map[string]interface{})
					if ok {
						collected = append(collected, repAgent)
					}
				}
			}
		}
	}
	result := make(map[string]interface{})
	for _, repAgent := range collected {
		if name, ok := repAgent["name"]; ok {
			result[name.(string)] = repAgent
		}
	}
	return result
}

func (ac *AEMDeploymentController) checkPublishConfig(publishPods []*v1.Pod, deployment *aemv1beta1.AEMDeployment) error {
	for _, p := range publishPods {
		c := aemconfig.Client{}
		pwd, _ := ac.getPodPassword(p, deployment.Name)
		dispatcherName := strings.Replace(p.Name, "-publish-", "-dispatcher-", -1)
		agent := aemconfig.NewAgentPublish(dispatcherName, aemconfig.PolicyCreate)
		agent.With = map[string]interface{}{
			"jcr:title": dispatcherName,
			"grid":      true,
			"enabled":   true,
			"protocolHTTPHeaders": []string{
				"CQ-Action:{action}",
				"CQ-Handle:{path}",
				"CQ-Path: {path}",
			},
			"protocolHTTPMethod": "GET",
			"retryDealy":         "60000",
			"serializationType":  "flush",
			"triggerReceive":     "true",
			"triggerSpecific":    "true",
			"noVersioning":       "true",
			"logLevel":           "error",
			"transportUri":       fmt.Sprintf("http://%s.%s.%s:80/dispatcher/invalidate.cache", dispatcherName, p.Spec.Subdomain, p.Namespace),
		}
		c.RegisterAgent(agent)
		// TODO: validate output
		_, err := c.Do(p.Status.PodIP, "4503", "admin", pwd)
		if err != nil {
			fmt.Println("Error registering dispatcher", err)
			return err
		}
	}
	return nil
}

func (ac *AEMDeploymentController) checkAuthorConfig(pod *v1.Pod, publishPods []*v1.Pod, deployment *aemv1beta1.AEMDeployment) error {
	ac.logger.Info("Author configuring starts")
	pwd, err := ac.getPodPassword(pod, deployment.Name)
	if err != nil {
		return err
	}
	listClient := aemconfig.Client{}
	listClient.RegisterAgent(aemconfig.NewAgentAuthor("", aemconfig.PolicyShow))
	out, err := listClient.Do(pod.Status.PodIP, "4502", "admin", pwd)
	if err != nil {
		fmt.Println("Error listing agents", err)
		return err
	}
	existingAgents := parseAgentsList(out.Data.Agents, "agents.author")

	c := aemconfig.Client{}
	// desiredAgents holds the name of the publishes available in the deployment
	desiredAgents := []string{}

	for _, p := range publishPods {
		desiredAgents = append(desiredAgents, p.Name)
		if _, ok := existingAgents[p.Name]; ok {
			continue
		}
		agent := aemconfig.NewAgentAuthor(p.Name, aemconfig.PolicyCreate)
		pPwd, _ := ac.getPodPassword(p, deployment.Name)
		agent.With = map[string]interface{}{
			"jcr:title":         p.Name,
			"grid":              true,
			"enabled":           true,
			"transportUser":     "admin",
			"transportPassword": pPwd,
			"port":              4503,
			"transportUri":      fmt.Sprintf("http://%s.%s:4503/bin/receive?sling:authRequestLogin=1", p.Spec.Hostname, p.Spec.Subdomain),
		}
		c.RegisterAgent(agent)
	}
	if len(c.Agents) > 0 {
		// TODO: Validate
		_, err = c.Do(pod.Status.PodIP, "4502", "admin", pwd)
		if err != nil {
			return err
		}
	}

	// Cleanup existing agents
	// check if agents contain grid=true value and then check if the agent exists in the desiredAgents list
	// otherwise the agent will be removed from the author
	dc := aemconfig.Client{}
	for agentName, agentMap := range existingAgents {
		agentValues, ok := agentMap.(map[string]interface{})
		if !ok {
			continue
		}
		value, ok := agentValues["grid"]
		if value == nil || !ok {
			continue
		}
		isGrid, _ := value.(bool)
		if isGrid {
			if !isInSlice(agentName, desiredAgents) {
				agent := aemconfig.NewAgentAuthor(agentName, aemconfig.PolicyDelete)
				dc.RegisterAgent(agent)
			}
		}
	}
	if len(dc.Agents) > 0 {
		_, err = dc.Do(pod.Status.PodIP, "4502", "admin", pwd)
		if err != nil {
			return err
		}
		ac.logger.Infof("Deleting [%v] agent(s) in author", len(dc.Agents))
	}

	ac.logger.Info("Author configured")
	return nil
}

func (ac *AEMDeploymentController) checkPodInitialized(pod *v1.Pod, deployment *aemv1beta1.AEMDeployment) error {
	pwd, err := ac.getPodPassword(pod, deployment.Name)
	if err != nil {
		fmt.Println("error getting previous password", err)
		return err
	}
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = make(map[string]string)
	}
	_, ok := pod.ObjectMeta.Annotations[podInitializedAnnotation]
	if !ok || pod.Annotations[podInitializedAnnotation] != "true" {

		if len(pwd) == 0 {
			pwd, _ = passwordGenerator.GeneratePassword(20, false)
			podSecretsKey := getPodSecretKey(pod.Namespace, deployment.Name, pod.Name)
			err := ac.secrets.Put(podSecretsKey, map[string]interface{}{"password": pwd})
			if err != nil {
				fmt.Println("error stroging secrets", err)
				return err
			}
		}
		c := aemconfig.Client{}
		user := aemconfig.NewUserChangePassword("admin", pwd)
		repDel := aemconfig.NewAgentAuthor("publish", aemconfig.PolicyDelete)
		c.RegisterAgent(repDel)
		c.RegisterUser(user)
		port := "4502"
		if isPublish(pod) {
			port = "4503"
		}
		_, err := c.Do(pod.Status.PodIP, port, "admin", "admin")
		if err != nil {
			fmt.Println("error setting password with default password", err)
		}

		// Retry with previous pwd
		_, err = c.Do(pod.Status.PodIP, port, "admin", pwd)
		if err != nil {
			fmt.Println("error setting password", err)
			return err
		}
		ac.logger.Infof("Password set for pod %s/%s", pod.Namespace, pod.Name)
		pod.Annotations[podInitializedAnnotation] = "true"
		_, err = ac.clientSet.CoreV1().Pods(pod.Namespace).Update(pod)
		if err != nil {
			ac.logger.Error("Error setting pod", err)
			return err
		}
	}
	return nil
}

func (ac *AEMDeploymentController) resizeDeployment(pods []*v1.Pod, desired int, deployment *aemv1beta1.AEMDeployment, runmode string) {
	actual := len(pods)
	// Grow action
	if actual < desired {
		// populates with the desired pods names
		desiredPods := []string{}
		for i := 1; i <= desired; i++ {
			id := fmt.Sprintf("%03d", i)
			desiredPods = append(desiredPods, k8s.MakePodName(deployment.Name, runmode, id))
		}
		// populates with actual pods names
		podsNames := []string{}
		for _, pod := range pods {
			podsNames = append(podsNames, pod.Name)
		}
		// creates the missing pods
		for _, podName := range desiredPods {
			if !isInSlice(podName, podsNames) {
				ac.addInstance(podName, runmode, deployment)
			}
		}
		return
	}
	// Shrink action
	if desired < actual {
		sort.Sort(ascendingOrdinal(pods))
		total := actual - desired
		removed := 0
		for last := actual - 1; removed < total; last = last - 1 {
			ac.removeInstance(pods[last], deployment)
			removed = removed + 1
		}
	}
}

// LabelsForDeployment returns the map of labels for an AEM deployment.
func LabelsForDeployment(deploymentName string) map[string]string {
	return map[string]string{
		"deployment": deploymentName,
		"app":        "aem",
	}
}

// GetPods filtered by func filter.
func GetPods(pods []*v1.Pod, filter func(pod *v1.Pod) bool) []*v1.Pod {
	var filteredPods []*v1.Pod
	for _, pod := range pods {
		if filter(pod) {
			filteredPods = append(filteredPods, pod)
		}
	}
	return filteredPods
}
