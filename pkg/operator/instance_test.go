package operator

import (
	"testing"
	"time"

	aemv1beta1 "github.com/xumak-grid/aem-operator/pkg/apis/aem/v1beta1"
	"github.com/xumak-grid/aem-operator/pkg/k8s"
	"github.com/xumak-grid/aem-operator/pkg/retry"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestAddAuthorInstance(t *testing.T) {
	client := fakeclientset.NewSimpleClientset()
	ns := "default"
	deployment := &aemv1beta1.AEMDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testing",
			Namespace: ns,
		},
	}
	aemc := getAEMDeploymentController(client)
	done := make(chan interface{})
	go func() {
		fakeBoundPVC(client, ns, k8s.MakePVCName("author"))
		close(done)
	}()
	aemc.addInstance("author", "author", deployment)
	<-done
	pods, _ := client.CoreV1().Pods(ns).List(metav1.ListOptions{})
	if len(pods.Items) != 1 {
		t.Error("Should have 1 pod")
	}
	pod := pods.Items[0]
	if !isAuthor(&pod) {
		t.Error("Should be an author instance")
	}
	if pod.Namespace != ns {
		t.Error("Should have the same namespace")
	}
}

func getLogger() *zap.Logger {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	l, _ := config.Build(zap.Fields(zap.String("operator_version", "testing_version")))
	return l
}

// typically when adding new instances the operator will wait until the claim is bound
// we need to fake it in order to  pass that checkpoint.
func fakeBoundPVC(cli kubernetes.Interface, ns, pvc string) {
	retry.Retry(300*time.Millisecond, 15, func() (bool, error) {
		var err error
		claim, err := cli.CoreV1().PersistentVolumeClaims(ns).Get(pvc, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		claim.Status.Phase = v1.ClaimBound
		cli.CoreV1().PersistentVolumeClaims(ns).Update(claim)
		return true, nil
	})
}

func getAEMDeploymentController(kubecli kubernetes.Interface) *AEMDeploymentController {
	return &AEMDeploymentController{
		logger:    getLogger().Sugar(),
		clientSet: kubecli,
	}
}
