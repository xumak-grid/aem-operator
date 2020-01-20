package operator

import (
	"os"
	"time"

	aemv1beta1 "github.com/xumak-grid/aem-operator/pkg/apis/aem/v1beta1"

	aemclientset "github.com/xumak-grid/aem-operator/pkg/generated/clientset/versioned"
	aeminformers "github.com/xumak-grid/aem-operator/pkg/generated/informers/externalversions/aem/v1beta1"
	"github.com/xumak-grid/aem-operator/pkg/k8s"
	"github.com/xumak-grid/aem-operator/pkg/secrets"
	vault "github.com/xumak-grid/aem-operator/pkg/secrets/vault"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// AEMDeploymentController represents the main operator controller.
type AEMDeploymentController struct {
	logger *zap.SugaredLogger
	// Clientset that has  REST client for interacting with k8s resources in the API Group.
	clientSet kubernetes.Interface
	// RESTClient for mananging specific requests that are not part of the official resources.
	restClient rest.Interface
	// AEM clientset
	aemcli      aemclientset.Interface
	kubeconfig  *rest.Config
	aemInformer cache.SharedIndexInformer
	podInformer coreinformers.PodInformer
	queue       workqueue.RateLimitingInterface
	secrets     secrets.SecretService
}

// NewAEMController creates a new controller for the AEM Operator.
func NewAEMController(kubeconfig string, logger *zap.Logger) (*AEMDeploymentController, error) {
	cfg, err := k8s.BuildKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	aemcli, _ := aemclientset.NewForConfig(cfg)

	sharedInformers := informers.NewSharedInformerFactory(clientSet, 15*time.Second)
	secrets, err := vault.NewSecretService()
	if err != nil {
		return nil, err
	}
	aemc := &AEMDeploymentController{
		kubeconfig:  cfg,
		podInformer: sharedInformers.Core().V1().Pods(),
		logger:      logger.Sugar(),
		clientSet:   clientSet,
		aemcli:      aemcli,
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "aemdeployment"),
		secrets:     secrets,
	}
	aemc.aemInformer = aemc.newAEMControllerInformer()
	aemc.aemInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    aemc.handleAddDeployment,
		DeleteFunc: aemc.handleDeleteDeployment,
		UpdateFunc: aemc.handleUpdateDeployment,
	})

	return aemc, nil
}

func (ac *AEMDeploymentController) newAEMControllerInformer() cache.SharedIndexInformer {
	ns := v1.NamespaceAll
	if len(os.Getenv("DEV_OPERATOR_NAMESPACE")) > 0 {
		ns = os.Getenv("DEV_OPERATOR_NAMESPACE")
	}
	resyncPeriod := 15 * time.Second
	return aeminformers.NewAEMDeploymentInformer(ac.aemcli, ns, resyncPeriod, cache.Indexers{})
}

// Run runs the controller.
func (ac *AEMDeploymentController) Run(stop <-chan struct{}) {
	defer ac.queue.ShutDown()
	// Run informers.
	go ac.aemInformer.Run(stop)
	go ac.podInformer.Informer().Run(stop)
	// Wait for informers to be ready.
	if !cache.WaitForCacheSync(stop, ac.aemInformer.HasSynced, ac.podInformer.Informer().HasSynced) {
		ac.logger.Error("time out while waiting for cache sync")
	}
	ac.logger.Info("cache synced")
	go ac.worker()
	<-stop
}

func (ac *AEMDeploymentController) handleAddDeployment(obj interface{}) {
	dep, ok := obj.(*aemv1beta1.AEMDeployment)
	if !ok {
		ac.logger.Info("Invalid deployment object")
		return
	}
	ac.enqueue(dep)
}

func (ac *AEMDeploymentController) handleUpdateDeployment(obj interface{}, newObj interface{}) {
	_, ok := obj.(*aemv1beta1.AEMDeployment)
	newDep, nok := obj.(*aemv1beta1.AEMDeployment)
	if !ok || !nok {
		ac.logger.Info("Invalid deployment object")
		return
	}
	// TODO: Remove this restriction after POD watching.
	// if dep.Metadata.ResourceVersion == newDep.Metadata.ResourceVersion {
	// 	// Periodic resyncs sends update events for all known deployments
	// 	// Equal resource versions means haven't had any changes since k8s updates the version with each update.
	// 	return
	// }
	ac.enqueue(newDep)
}
func (ac *AEMDeploymentController) handleDeleteDeployment(obj interface{}) {
	dep, ok := obj.(*aemv1beta1.AEMDeployment)
	if !ok {
		ac.logger.Info("Invalid object ")
		// tombstone, ok := obj.(cache.DeletedFinalStateUnknown) https://github.com/coreos/etcd-operator/pull/1386/files
		return
	}
	path := getSecretBasePath(dep.Namespace, dep.Name)
	ac.logger.Infof("Cleaning secrets for path: %v", path)
	err := ac.secrets.CleanUp(path)
	if err != nil {
		ac.logger.Errorf("error cleaning up secrets: %v", err.Error())
		return
	}
	ac.enqueue(dep)
}

// keyFunc generates a key based on namespace/name for objects implementing meta.Interface
func (ac *AEMDeploymentController) keyFunc(object interface{}) (string, bool) {
	k, err := cache.DeletionHandlingMetaNamespaceKeyFunc(object)
	if err != nil {
		ac.logger.Info("Error creating key failed")
		return k, false
	}
	return k, true
}

func (ac *AEMDeploymentController) enqueue(obj interface{}) {
	if obj == nil {
		return
	}
	key, ok := obj.(string)
	if !ok {
		key, ok = ac.keyFunc(obj)
		if !ok {
			return
		}
	}
	ac.queue.Add(key)
}

func (ac *AEMDeploymentController) worker() {
	for ac.processNextWorkItem() {
	}
}
func (ac *AEMDeploymentController) processNextWorkItem() bool {
	key, quit := ac.queue.Get()
	if quit {
		return false
	}
	defer ac.queue.Done(key)
	ac.logger.Infof("Processing %s", key)
	err := ac.sync(key.(string))
	if err == nil {
		ac.queue.Forget(key)
		return true
	}
	ac.queue.AddRateLimited(key)
	return true
}
