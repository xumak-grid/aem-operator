package k8s

import (
	"fmt"
	"path"
	"time"

	aemv1beta1 "github.com/xumak-grid/aem-operator/pkg/apis/aem/v1beta1"
	"github.com/xumak-grid/aem-operator/pkg/retry"
	"k8s.io/api/core/v1"
	v1beta1storage "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var defaultStorageClass = "gp2"

const (
	storageClassPrefix    = "aem-backup"
	backupPVVolName       = "aem-backup-storage"
	awsCredentialDir      = "/root/.aws/"
	awsConfigDir          = "/root/.aws/config/"
	awsSecretVolName      = "secret-aws"
	awsConfigVolName      = "config-aws"
	fromDirMountDir       = "/mnt/backup/from"
	defaultVolumeSizeInMB = 1024 * 10 // 10 GiB

)

// CreateAndWaitPVC creates a volume claim for an instance.
func CreateAndWaitPVC(cli kubernetes.Interface, instanceName string, deployment *aemv1beta1.AEMDeployment) error {
	name := MakePVCName(instanceName)
	claim := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"deployment": instanceName,
				"app":        "aem",
			},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &defaultStorageClass,
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dMi", defaultVolumeSizeInMB)),
				},
			},
		},
	}
	if deployment.AsOwnerReference() != nil {
		claim.OwnerReferences = append(claim.OwnerReferences, *deployment.AsOwnerReference())
	}
	_, err := cli.CoreV1().PersistentVolumeClaims(deployment.Namespace).Create(claim)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	// Wait until the Claim is Bound.
	err = retry.Retry(500*time.Millisecond, 10, func() (bool, error) {
		var err error
		claim, err = cli.CoreV1().PersistentVolumeClaims(deployment.Namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if claim.Status.Phase != v1.ClaimBound {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		wErr := fmt.Errorf("fail to wait PVC (%s) '(%v)/Bound': %v", name, claim.Status.Phase, err)
		return wErr
	}
	return nil
}

// MakePVCName returns a desired name of the persistent volume claim
func MakePVCName(podName string) string {
	return fmt.Sprintf("%s-pvc", podName)
}

// makeVolumeKey returns deploymentName-key string
func makeVolumeKey(deploymentName, key string) string {
	return fmt.Sprintf("%s-%s", deploymentName, key)
}

// CreateStorageClass handles storage class creation, specially needed
// for Retention Policy
func CreateStorageClass(kubecli kubernetes.Interface, pvProvisioner string) error {
	// We need to get rid of prefix because naming doesn't support "/".
	name := storageClassPrefix + "-" + path.Base(pvProvisioner)
	class := &v1beta1storage.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Provisioner: pvProvisioner,
		// See https://kubernetes.io/docs/concepts/storage/persistent-volumes/#parameters
		Parameters: map[string]string{
			"Type": "gp2",
		},
	}
	_, err := kubecli.StorageV1beta1().StorageClasses().Create(class)
	return err
}
