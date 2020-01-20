.PHONY: run test build compile image deploy
TAG?=$(shell git rev-parse --short HEAD)
REPO?=/grid/aem-operator

default: test run-dev
run-dev:
	go run cmd/aem-operator/*.go --kubeconfig $(HOME)/.kube/config

run:
	go run cmd/aem-operator/*.go

test:
	go test -cover github.com/xumak-grid/aem-operator/pkg/k8s
	go test -cover github.com/xumak-grid/aem-operator/pkg/operator
	go test -cover github.com/xumak-grid/aem-operator/pkg/secrets/vault

build: 
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o bin/operator  -tags netgo cmd/aem-operator/*.go

compile:
	mkdir -p bin
	cd cmd/operator; CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o ../../bin/aem-operator -a -tags netgo -ldflags "-X github.com/xumak-grid/aem-operator/version.GitSHA=$(TAG)" .

image:
	docker build -t $(REPO):$(TAG) .
	docker tag $(REPO):$(TAG) $(REPO):latest

deploy: image
	docker push $(REPO):$(TAG)
	docker push $(REPO):latest

aem-simulator:
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o cmd/aem-simulator/bin/aem-simulator -a -tags netgo cmd/aem-simulator/*.go
	docker build -t /grid/aem-simulator:latest  cmd/aem-simulator/
	rm -rf cmd/aem-simulator/bin

aem-operator:
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o cmd/aem-operator/bin/operator -a -tags netgo cmd/aem-operator/*.go
	docker build -t grid/aem-operator  cmd/aem-operator/
	rm -rf cmd/aem-operator/bin

build-minikube: 
	./hack/minikube.sh

run-minikube:
	./hack/run-minikube.sh

minikube-vault-install:
	helm install incubator/vault --set vault.dev=false --name dev --set image.tag=0.8.3 --set vault.config.storage.file.path=/root/data

minikube-start:
	minikube start --vm-driver xhyve --mount --mount-string $(PWD):/development/aem-operator --memory 8192 --cpus 6 

minikube-run:
	 kubectl -it exec aem-operator-dev /bin/sh

minikube-mount:
	minikube mount $(PWD):/development/aem-operator
