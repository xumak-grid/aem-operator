package k8s

import "testing"

func TestPVCName(t *testing.T) {
	name := "aem-author-001"
	pvcName := "aem-author-001-pvc"
	if MakePVCName(name) != pvcName {
		t.Error("Should be equal")
	}
}
