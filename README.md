# mailcow-operator

Kubernetes operator for managing mailcow resources with Custom Resource Definitions (CRDs). It reconciles `Mailcow`, `Domain`, `Mailbox`, `Alias`, and `DomainAdmin` resources.

## Features

- Declarative management of mailcow domains, mailboxes, aliases, and domain admins
- Finalizers to ensure clean deletion


## Prerequisites

- Kubernetes cluster
- mailcow deployment reachable from the operator
- helm (for installation via Helm)

## Install via Helm

The Helm chart lives in [helm/chart](helm/chart).

1. Install CRDs and controller:

```bash
helm install mailcow-operator ./helm/chart \
	--namespace mailcow-operator \
	--create-namespace
```

or using the released version:

```bash
helm repo add mailcow-operator https://tarteo.github.io/mailcow-operator
helm install mailcow-operator mailcow-operator/mailcow-operator \
    --namespace mailcow-operator \
    --create-namespace
```

3. Uninstall:

```bash
helm uninstall mailcow-operator --namespace mailcow-operator
```

## Using the CRDs

The operator manages these CRDs:

- `Mailcow` — stores API endpoint and credentials reference
- `Domain` — manages mail domains
- `Mailbox` — manages mailboxes for domains
- `Alias` — manages aliases
- `DomainAdmin` — manages domain administrators

### Create a Mailcow resource

```yaml
apiVersion: mailcow.onestein.nl/v1
kind: Mailcow
metadata:
  name: example-mailcow
spec:
  endpoint: "https://mail.example.com"
  secret:
    name: mailcow-credentials
    key: apiToken
```

### Create a Domain

```yaml
apiVersion: mailcow.onestein.nl/v1
kind: Domain
metadata:
  name: example-domain
spec:
  mailcow: example-mailcow
  domain: "example.com"
  description: "Example Domain"
  quota: 1000
  defQuota: 500
  maxQuota: 500
  active: true
  maxMailboxes: 60
```

### Create a Mailbox

```yaml
apiVersion: mailcow.onestein.nl/v1
kind: Mailbox
metadata:
  name: example-mailbox
spec:
  mailcow: example-mailcow
  domain: example.com
  localPart: "user"
  name: "mr. example"
  passwordSecret:
    name: mailbox-password-secret
    key: password
  quota: 500
  active: true
```

### Create an Alias

```yaml
apiVersion: mailcow.onestein.nl/v1
kind: Alias
metadata:
  name: example-alias
spec:
  mailcow: example-mailcow
  address: "@example.com" # Catch-all alias
  goTo: "user@example.com"
  active: true
```

### Create a DomainAdmin

```yaml
apiVersion: mailcow.onestein.nl/v1
kind: DomainAdmin
metadata:
  name: example-domainadmin
spec:
  mailcow: example-mailcow
  username: "test-example"
  passwordSecret:
    name: domainadmin-password-secret
    key: password
  domains:
    - example.com
    - example2.com
  active: true
```

## Development

### Generate CRDs and deepcopy

```bash
make manifests generate
```

### Build and run locally

```bash
make build
make run
```

### Regenerate mailcow API client

The mailcow API is generated from the [mailcow OpenAPI specification](mailcow/openapi.yaml) using [oapi-codegen](https://github.com/deepmap/oapi-codegen).
The openapi.yaml is pulled from a mailcow version 2024-01d release. And then edited due to the specification not being fully correct and missing schemas for some endpoints.

```bash
oapi-codegen --config=mailcow/oapi-codegen.yaml mailcow/openapi.yaml
```

## Roadmap

- Update status conditions
- Add more controllers for other mailcow resources
- Add e2e tests
- Allow multiple goto addresses in aliases
- Make the created DKIM ConfigMap name configurable
- Add an option to force the password of mailbox resources to be updated on each reconciliation according to the secret
- Add support for multiple mailcow versions
