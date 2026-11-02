# BlanketOps Environments HTTP Handler

REST API server for BlanketOps Environments — serves the Kubernetes API and embedded UI from a single binary.

## Overview

The environments-server provides a unified HTTP API for managing BlanketOps Environments resources (Builds, GitHubEvents, Environments, Deployments, Packages, ServiceUnits, Routes, Domains, GitRepositories) and serves the embedded React UI. It uses the Kubernetes client-go dynamic client to interact with the cluster, requiring only a kubeconfig.

## Features

- **REST API** — Full CRUD for all Environments CRD kinds via `/api/v1/{kind}` endpoints
- **Embedded UI** — Serves the Environments React UI from the same origin (no nginx proxy needed)
- **Kubernetes-native** — Uses controller-runtime client with dynamic scheme registration
- **CORS enabled** — Ready for browser consumption
- **Health endpoint** — `/health` for liveness/readiness probes
- **ko build** — Multi-arch container images via ko

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│               environments-http-handler                     │
├─────────────────────────────────────────────────────────────┤
│  GET  /health              → k8s connectivity check          │
│  GET  /api/v1/{kind}       → List resources (with ?namespace=)│
│  POST /api/v1/{kind}       → Create resource                 │
│  GET  /api/v1/{kind}/{ns}/{name}  → Get resource             │
│  PATCH /api/v1/{kind}/{ns}/{name} → Patch resource           │
│  DELETE /api/v1/{kind}/{ns}/{name} → Delete resource         │
│  GET  /api/v1/{kind}/{ns}/{name}/events → Resource events    │
│  GET  /api/v1/{kind}/{ns}/{name}/debug  → Debug info         │
│  GET  /                    → Embedded React UI (if built)     │
└─────────────────────────────────────────────────────────────┘
```

## Supported Kinds

- `builds` (environments.blanketops.dev/v1alpha1)
- `githubevents` (events.blanketops.dev/v1alpha1)
- `environments` (environments.blanketops.dev/v1alpha1)
- `deployments` (environments.blanketops.dev/v1alpha1)
- `packages` (environments.blanketops.dev/v1alpha1)
- `serviceunits` (environments.blanketops.dev/v1alpha1)
- `routes` (networks.blanketops.dev/v1alpha1)
- `domains` (networks.blanketops.dev/v1alpha1)
- `gitrepositories` (sources.blanketops.dev/v1alpha1)

## Quick Start

### Prerequisites

- Go 1.26+
- Kubernetes cluster (kind, k3s, EKS, GKE, etc.)
- ko (for container builds)

### Local Development

```bash
# Run against local cluster (uses ~/.kube/config)
go run .

# Or specify kubeconfig
KUBECONFIG=/path/to/kubeconfig go run .

# Server starts on :8082
curl http://localhost:8082/health
# {"status":"ok","connected":true,"store":"kubernetes"}
```

### Build Container Image

```bash
# Build and push to ghcr.io (requires ko and auth)
ko publish --platform=linux/amd64,linux/arm64 .
```

## Configuration

| Env Var        | Default            | Description        |
| -------------- | ------------------ | ------------------ |
| `PORT`       | `8082`           | HTTP listen port   |
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig |

## API Examples

### List Builds

```bash
curl http://localhost:8082/api/v1/builds
# {"items":[...],"total":5}
```

### List Builds in Namespace

```bash
curl "http://localhost:8082/api/v1/builds?namespace=production"
```

### Get Build

```bash
curl http://localhost:8082/api/v1/builds/default/my-build
```

### Create Build

```bash
curl -X POST http://localhost:8082/api/v1/builds \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {"name": "my-build", "namespace": "default"},
    "spec": {
      "image": "ghcr.io/org/app:latest",
      "source": {"url": "https://github.com/org/repo.git", "revision": "main"},
      "strategy": {"name": "kaniko", "kind": "ClusterBuildStrategy"}
    }
  }'
```

### Health Check

```bash
curl http://localhost:8082/health
# {"status":"ok","connected":true,"store":"kubernetes"}
```

## Deployment

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: environments-server
  namespace: blanketops-environments-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: environments-server
  template:
    metadata:
      labels:
        app: environments-server
    spec:
      serviceAccountName: environments-server
      containers:
      - name: server
        image: ghcr.io/blanketops/environments-server:latest
        ports:
        - containerPort: 8082
        env:
        - name: PORT
          value: "8082"
        livenessProbe:
          httpGet:
            path: /health
            port: 8082
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8082
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: environments-server
  namespace: blanketops-system
spec:
  selector:
    app: environments-server
  ports:
  - port: 80
    targetPort: 8082
```

### RBAC (ClusterRole)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: environments-server
rules:
- apiGroups: ["environments.blanketops.dev", "events.blanketops.dev", "networks.blanketops.dev", "sources.blanketops.dev"]
  resources: ["*"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["namespaces", "secrets", "serviceaccounts"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: environments-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: environments-server
subjects:
- kind: ServiceAccount
  name: environments-server
  namespace: blanketops-system
```

## UI Integration

The server serves the embedded Environments React UI when built with ko (the UI assets are bundled as kodata). The UI is available at `/` and proxies `/api/*` calls to the server itself — no separate nginx proxy needed.

### Local UI Development

```bash
# Terminal 1: Run API server
go run .

# Terminal 2: Run UI dev server (proxies /api to :8082)
cd ../environments-ui
npm run dev -- --port 5173
```

## Project Structure

```
.
├── main.go                 # HTTP server, routing, handlers
├── go.mod / go.sum         # Go module
├── .ko.yaml                # ko build config (multi-arch)
├── Dockerfile              # Alternative container build
├── internal/
│   └── types/              # API request/response types
├── wasm/                   # WASM build artifacts (if any)
├── hack/                   # Build scripts
└── .github/workflows/      # CI/CD pipelines
```

## CI/CD

See `.github/workflows/` for:

- **ci.yml** — Go test, vet, build on PR/push
- **release.yml** — ko publish on tag push
- **docker.yml** — Alternative Docker build

## License

Apache-2.0 — see [LICENSE](LICENSE)
