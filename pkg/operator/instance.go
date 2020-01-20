package operator

import (
	aemv1beta1 "github.com/xumak-grid/aem-operator/pkg/apis/aem/v1beta1"
	"github.com/xumak-grid/aem-operator/pkg/k8s"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ac *AEMDeploymentController) addInstance(name, runmode string, deployment *aemv1beta1.AEMDeployment) error {
	pod := k8s.NewPod(name, runmode, deployment)
	err := k8s.CreateAndWaitPVC(ac.clientSet, name, deployment)
	if err != nil {
		ac.logger.Error("Error creating PVC for", err)
		return err
	}
	_, err = ac.clientSet.CoreV1().Pods(deployment.Namespace).Create(pod)
	if err != nil {
		ac.logger.Error("Error creating pod", err)
		return err
	}
	err = k8s.CreateExternalEndpoint(ac.clientSet, name, runmode, deployment)
	if err != nil {
		ac.logger.Error("Error creating external endpoint", err)
		return err
	}
	return nil
}

// removeInstance makes a cleanup deleting resources when a pod is deleted
func (ac *AEMDeploymentController) removeInstance(pod *v1.Pod, deployment *aemv1beta1.AEMDeployment) error {
	ns := deployment.Namespace
	if isAuthor(pod) || isPublish(pod) {
		pwd, _ := ac.getPodPassword(pod, deployment.Name)
		if pwd != "" {
			ac.secrets.Delete(getPodSecretKey(ns, deployment.Name, pod.Name))
		}
	}
	deleteOptions := &metav1.DeleteOptions{}
	ac.clientSet.ExtensionsV1beta1().Ingresses(ns).Delete(k8s.MakeIngressName(pod.Name), deleteOptions)
	ac.clientSet.CoreV1().Services(ns).Delete(k8s.MakeServiceName(pod.Name), deleteOptions)
	ac.clientSet.CoreV1().PersistentVolumeClaims(ns).Delete(k8s.MakePVCName(pod.Name), deleteOptions)
	ac.clientSet.CoreV1().Pods(ns).Delete(pod.Name, deleteOptions)
	return nil
}
