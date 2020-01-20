# Adobe AEM Operator

## Overview

The Adobe AEM operator manages AEM deployments of authors, publishers, and dispatchers deployed to
[Kubernetes][k8s-home] and automates tasks related to operating a deployemnt.

## Requirements

* Kubernetes 1.9+
* Adobe AEM 6.3+

## Create and destroy an Adobe AEM deployment

```bash
$ kubectl create namespace demo
$ kubectl create -f example/example-aem-deployment.yaml
```

A 3 member deployment will be created.

```bash
$ kubectl get pods
NAME                 READY     STATUS    RESTARTS   AGE
dev-author-001       1/1       Running   0          6h
dev-dispatcher-001   1/1       Running   6          6h
dev-publish-001      1/1       Running   0          6h
```

## Limitations

* AWS Support only (for now)

## Dependencies

Get dependencies

```bash
dep ensure -vendor-only -v
```

## Development

* Go 1.9+
* [dep][golang-dep]

### Setup

In development the aem-operator needs to have access to Vault server, authos, publishers and disptachers.

- For Vault you can have access via port-forward the active Vault pod
```sh
# to get de active pod
activePod=$(kubectl -n bedrock get vault grid-vault -o jsonpath='{.status.vaultStatus.active}')
kubectl -n bedrock port-forward $activePod 8200
```

- For authos, publishers and disptachers IPs via VPN access: create a vpn configuration file in `xumak-grid/k8s-clusters/test/setup-vpn.sh`

Export the following environment vars:

```sh
# The namespace the operator will listen
export DEV_OPERATOR_NAMESPACE=ehernandez
# Vault config
export VAULT_ADDR=https://127.0.0.1:8200
export VAULT_TOKEN=SECRET-TOKEN-HERE
export VAULT_SKIP_VERIFY=true
# External DNS for grid, the following is for 'test' cluster
export GRID_EXTERNAL_DOMAIN=test.grid.xumak.io

# Run the operator
make
```

[k8s-home]: https://kubernetes.io
[golang-dep]: https://github.com/golang/dep

Copyright © 2016 Tikal Technologies, Inc.