# Shawarma

[![ci](https://github.com/CenterEdge/shawarma/actions/workflows/docker-image.yml/badge.svg)](https://github.com/CenterEdge/shawarma/actions/workflows/docker-image.yml)

A Kubernetes sidecar to assist with enabling/disabling background processing during blue/green
deployments.

## Overview

A [Blue/Green Deployment](https://martinfowler.com/bliki/BlueGreenDeployment.html) is a process
designed to maintain 100% uptime during deployments, with rapid rollbacks. As the new version
is deployed, traffic is routed to the new version and diverted from the old version. However,
the old version is left running and ready to receive traffic, allowing for rapid failover to
the previous version in the event a rollback is required.

This works great for serving incoming requests, but what about background processes running
within the application? For example, running scheduled background jobs or processing messages
from the message bus. In a traditional blue/green deployment, these processes continue to
execute, potentionally leaving a bug operating in production that you thought you fixed.

## How it Works

Shawarma is designed to address this problem for applications running within Kubernetes.
It is a very lightweight Go app which runs in a sidecar container within each pod of your
application. It monitors the Kubernetes API to know when the pod is or is not connected to
the load balancer, and uses an HTTP POST to let your application know the state. Your
application must simply receive the POST and start or stop background processing.

## Example

To see an example deployment utilizing Shawarma, see (./example/basic/example.yaml).

For a more automated example using annotations to automatically inject sidecars, see
(./example/injected).

## RBAC Rights

Shawarma requires access rights, via a service account, to monitor endpoints with the
pod's namespace. It is recommended to create a single Role named `shawarma'
in the namespace, and then bind it to each service account using a RoleBinding.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: shawarma
rules:
- apiGroups: [""]
  resources: ["endpoints"]
  verbs: ["get", "watch", "list"]
```

## Usage

`shawarma monitor [arguments...]`

Shawarma only functions within Kubernetes, using service tokens for authentication,
so normally it is run using the Docker container `centeredge/shawarma`.

For detailed help:

`docker run --rm -it centeredge/shawarma monitor --help`

## Arguments

Most arguments can be specified either on the command line, or via an environment variable.
If specified both places, the command line takes precendence.

| Name        | Env Var         | Description |
| ----------- | --------------- | ----------- |
| --log-level | LOG_LEVEL       | Set the log level (panic, fatal, error, warn, info, debug, trace) (default: `warn`) |
| --namespace | NAMESPACE       | Kubernetes namespace, typically a fieldRef to `fieldPath: metadata.namespace` (default: `default`) |
| --pod       | POD_NAME        | Kubernetes pod name, typically a fieldRef to `fieldPath: metadata.name` (no default) |
| --label     | ENDPOINT_LABEL  | The label value on the Kubernetes service/endpoint to monitor, maps to the label key `shawarma.centeredge.io/service-label` (no default) |
| --url       | SHAWARMA_URL    | URL which receives a POST request on state change (default: `http://localhost/applicationstate`) |
