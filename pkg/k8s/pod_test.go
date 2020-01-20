package k8s

import (
	"testing"

	"k8s.io/api/core/v1"
)

func containsPort(ports []v1.ContainerPort, port int) bool {
	for _, p := range ports {
		if int(p.ContainerPort) == port {
			return true
		}
	}
	return false
}

func TestAEMContainer(t *testing.T) {
	authorContainer := aemContainer(AEMRunmodeAuthor)

	if !containsPort(authorContainer.Ports, 4502) {
		t.Error("Should expose port 4502")
	}

	if !containsPort(authorContainer.Ports, 9010) {
		t.Error("Should expose jmx port 9010")
	}

	publishContainer := aemContainer(AEMRunmodePublish)
	if !containsPort(publishContainer.Ports, 4503) {
		t.Error("Should expose port 4503")
	}

	if !containsPort(publishContainer.Ports, 9010) {
		t.Error("Should expose jmx port 9010")
	}

}

func TestDispatcherContainer(t *testing.T) {
	dispatcherContainer := dispatcherContainer("exampleName", "example-deployment", "demo")
	if !containsPort(dispatcherContainer.Ports, 80) {
		t.Error("Should expose port 80")
	}

	if !containsPort(dispatcherContainer.Ports, 443) {
		t.Error("Should expose port 443")
	}
}

func TestSideContainer(t *testing.T) {
	sideCarContainer := dispatcherSideCar("example-deployment")
	if !containsPort(sideCarContainer.Ports, 9090) {
		t.Error("Should expose port 80")
	}
}

func TestMatchPublishHost(t *testing.T) {
	ns := "demo"
	deployName := "example-aem"
	table := []struct {
		dispatcherName string
		output         string
	}{
		{dispatcherName: "example-aem-dispatcher-001", output: "example-aem-publish-001.example-aem.demo"},
		{dispatcherName: "", output: "localhost"},
	}

	for _, i := range table {
		got := matchPublishHost(i.dispatcherName, deployName, ns)
		if got != i.output {
			t.Errorf("got: %v exected: %v", got, i.output)
		}
	}
}

func TestGetPodHost(t *testing.T) {
	table := []struct {
		pod     string
		service string
		ns      string
		output  string
	}{
		{pod: "example-aem-publish-001", service: "example-aem", ns: "demo", output: "example-aem-publish-001.example-aem.demo"},
	}
	for _, i := range table {
		got := GetPodHost(i.pod, i.service, i.ns)
		if got != i.output {
			t.Errorf("got: %v exected: %v", got, i.output)
		}
	}
}
