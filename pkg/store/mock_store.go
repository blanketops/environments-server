/*
Copyright 2026 The BlanketOps Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/blanketops/environments-server/pkg/types"
)

// MockStore is an in-memory, thread-safe store seeded with realistic
// environment CRs. It lets the server run and the UI be demonstrated
// without a live Kubernetes cluster.
type MockStore struct {
	mu   sync.RWMutex
	data map[string]*types.CRStatus // key: kind/namespace/name
}

func NewMockStore() (*MockStore, error) {
	m := &MockStore{data: make(map[string]*types.CRStatus)}
	if err := m.seed(); err != nil {
		return nil, err
	}
	slog.Info("mock store seeded", "items", len(m.data))
	return m, nil
}

func (m *MockStore) key(kind, ns, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, ns, name)
}

func (m *MockStore) List(kind, namespace string) ([]types.CRStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []types.CRStatus
	for _, item := range m.data {
		if item.Kind != kind {
			continue
		}
		if namespace != "" && item.Namespace != namespace {
			continue
		}
		cp := *item // shallow copy so callers cannot mutate stored state
		out = append(out, cp)
	}
	return out, nil
}

func (m *MockStore) Get(kind, namespace, name string) (types.CRStatus, error) {
	m.mu.RLock()
	item, ok := m.data[m.key(kind, namespace, name)]
	m.mu.RUnlock()
	if !ok {
		return types.CRStatus{}, &StoreError{Status: 404, Message: fmt.Sprintf("%s/%s not found", kind, name), Reason: "NotFound"}
	}
	return *item, nil
}

func (m *MockStore) Create(kind, defaultNS string, body types.CreateBody) (types.CRStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := body.Metadata.Name
	if name == "" {
		name = fmt.Sprintf("%s-%d", kind, time.Now().UnixMilli())
	}
	ns := body.Metadata.Namespace
	if ns == "" {
		ns = defaultNS
	}
	k := m.key(kind, ns, name)
	if _, exists := m.data[k]; exists {
		return types.CRStatus{}, &StoreError{Status: 409, Message: fmt.Sprintf("%s/%s already exists", kind, name), Reason: "AlreadyExists"}
	}

	item := types.CRStatus{
		Name:        name,
		Namespace:   ns,
		Kind:        kind,
		Phase:       types.PhasePending,
		Generation:  1,
		Conditions:  []types.Condition{},
		Events:      []types.CREvent{},
		Spec:        body.Spec,
		Labels:      body.Metadata.Labels,
		Annotations: body.Metadata.Annotations,
	}
	if item.Labels == nil {
		item.Labels = map[string]string{}
	}
	if item.Annotations == nil {
		item.Annotations = map[string]string{}
	}
	m.data[k] = &item
	slog.Info("mock create", "kind", kind, "name", name, "namespace", ns)
	return item, nil
}

func (m *MockStore) Patch(kind, namespace, name string, body types.PatchBody) (types.CRStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := m.key(kind, namespace, name)
	item, ok := m.data[k]
	if !ok {
		return types.CRStatus{}, &StoreError{Status: 404, Message: fmt.Sprintf("%s/%s not found", kind, name), Reason: "NotFound"}
	}
	if body.Spec != nil {
		item.Spec = body.Spec
	}
	if body.Metadata != nil {
		if body.Metadata.Labels != nil {
			if item.Labels == nil {
				item.Labels = map[string]string{}
			}
			for k2, v := range body.Metadata.Labels {
				item.Labels[k2] = v
			}
		}
		if body.Metadata.Annotations != nil {
			if item.Annotations == nil {
				item.Annotations = map[string]string{}
			}
			for k2, v := range body.Metadata.Annotations {
				item.Annotations[k2] = v
			}
		}
	}
	item.Generation++
	m.data[k] = item
	return *item, nil
}

func (m *MockStore) Delete(kind, namespace, name string) (types.DeleteResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := m.key(kind, namespace, name)
	if _, exists := m.data[k]; !exists {
		return types.DeleteResponse{}, &StoreError{Status: 404, Message: fmt.Sprintf("%s/%s not found", kind, name), Reason: "NotFound"}
	}
	delete(m.data, k)
	return types.DeleteResponse{Name: name, Namespace: namespace, Kind: kind, Deleted: true}, nil
}

func (m *MockStore) GetEvents(kind, namespace, name string) ([]types.CREvent, error) {
	m.mu.RLock()
	_, ok := m.data[m.key(kind, namespace, name)]
	m.mu.RUnlock()
	if !ok {
		return nil, &StoreError{Status: 404, Message: fmt.Sprintf("%s/%s not found", kind, name), Reason: "NotFound"}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return []types.CREvent{
		{Type: "Normal", Reason: "Created", Message: "Environment created", Count: 1, FirstTime: now, LastTime: now},
		{Type: "Normal", Reason: "Reconciled", Message: "Environment reconciled successfully", Count: 1, FirstTime: now, LastTime: now},
	}, nil
}

// GetLogs returns mock log lines for demonstration purposes.
func (m *MockStore) Logs(pod, namespace, container string, tailLines int64) (string, error) {
	m.mu.RLock()
	_, ok := m.data[m.key("pods", namespace, pod)]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("pod %s/%s not found", namespace, pod)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Mock logs for pod %s/%s, container %s (last %d lines):\n", namespace, pod, container, tailLines))
	sb.WriteString("2026/11/19 10:00:00 INFO starting controller\n")
	sb.WriteString("2026/11/19 10:00:01 INFO connected to kubernetes API\n")
	sb.WriteString("2026/11/19 10:00:05 INFO reconciler started\n")
	sb.WriteString("2026/11/19 10:00:10 INFO no resources to reconcile\n")
	return sb.String(), nil
}

// ListPods returns mock pod names for demonstration purposes.
func (m *MockStore) ListPods(namespace string) ([]string, error) {
	return []string{
		"blanketops-environments-controller-manager-86f6977b78-48jdl",
	}, nil
}

func (m *MockStore) seed() error {
	now := time.Now().UTC().Format(time.RFC3339)

	add := func(kind, name, ns, phase string, conditions []types.Condition, spec map[string]interface{}, labels map[string]string) error {
		specBytes, err := json.Marshal(spec)
		if err != nil {
			return err
		}
		item := types.CRStatus{
			Name:        name,
			Namespace:   ns,
			Kind:        kind,
			Phase:       phase,
			Generation:  1,
			Conditions:  conditions,
			Events: []types.CREvent{
				{Type: "Normal", Reason: "Created", Message: kind + " created", Count: 1, FirstTime: now, LastTime: now},
			},
			Spec:        specBytes,
			Labels:      labels,
			Annotations: map[string]string{},
		}
		m.data[m.key(kind, ns, name)] = &item
		return nil
	}

	// Real controller condition types, mirroring environments-controller
	// internal/controller/observers/environment/environment.go:
	//   Build.<name>.Ready      (BuildSuccess=True → true)
	//   GitRepository.<name>.Ready  (Ready=True → true)
	//   Deployment.<name>.Ready     (Ready=True → true)
	//   Route.<name>.Ready          (Ready=True → true)
	//   Package.<name>.Ready        (Succeeded=True → true)
	//   ServiceUnit.<name>.Ready    (ServiceUnitReady=True → true)
	envCondition := func(ready bool, kind, name, msg string) types.Condition {
		status := "False"
		if ready {
			status = "True"
		}
		return types.Condition{
			Type:               kind + "." + name + ".Ready",
			Status:             status,
			Reason:             kind,
			Message:            msg,
			LastTransitionTime: now,
		}
	}

	contract := func(appName, envType, version, branch, gitOwner, provider string, vault ...map[string]interface{}) map[string]interface{} {
		ss := map[string]interface{}{"provider": provider}
		if len(vault) > 0 {
			ss["vault"] = vault[0]
		}
		return map[string]interface{}{
			"applicationName": appName,
			"environmentType": envType,
			"version":         version,
			"branch":          branch,
			"gitOwner":        gitOwner,
			"secretStore":     ss,
		}
	}

	// ── Environments ──
	envs := []struct {
		name, ns, phase string
		conditions      []types.Condition
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "env-production", ns: "blanketops-environments", phase: "ENVIRONMENT_PHASE_READY",
			conditions: []types.Condition{
				envCondition(true, "Build", "build-payment-api", "Build applied successfully"),
				envCondition(true, "GitRepository", "payment-api-repo", "Repository ready"),
				envCondition(true, "Deployment", "deploy-payment-api", "Deployment rolled out"),
				envCondition(true, "Route", "route-payment-api", "Route bound"),
				envCondition(true, "Package", "pkg-cert-manager", "Package installed"),
				envCondition(true, "ServiceUnit", "svc-payment-worker", "Service unit healthy"),
			},
			spec: contract("payment-api", "PRODUCTION", "2.1.0", "main", "blanketops", "vault",
				map[string]interface{}{
					"address":   "http://vault:8200",
					"path":      "secret",
					"role":      "payments-role",
					"mountPath": "kubernetes",
					"version":   "v2",
				}),
			labels: map[string]string{"environments.blanketops.dev/name": "env-production", "environments.blanketops.dev/type": "production"},
		},
		{
			name: "env-staging", ns: "blanketops-environments", phase: "ENVIRONMENT_PHASE_READY",
			conditions: []types.Condition{
				envCondition(true, "Build", "build-orders-api", "Build applied successfully"),
				envCondition(true, "GitRepository", "orders-api-repo", "Repository ready"),
				envCondition(true, "Deployment", "deploy-orders-api", "Deployment rolled out"),
				envCondition(true, "Route", "route-orders-api", "Route bound"),
				envCondition(true, "Package", "pkg-ingress-nginx", "Package installed"),
				envCondition(true, "ServiceUnit", "svc-order-processor", "Service unit healthy"),
			},
			spec: contract("orders-api", "STAGING", "1.4.0", "develop", "blanketops", "aws"),
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging", "environments.blanketops.dev/type": "staging"},
		},
		{
			name: "env-development", ns: "blanketops-environments", phase: "ENVIRONMENT_PHASE_PENDING",
			conditions: []types.Condition{
				envCondition(false, "Build", "build-frontend", "Build is not True"),
				envCondition(false, "GitRepository", "", "no GitRepository condition"),
				envCondition(true, "Deployment", "deploy-frontend", "Deployment rolled out"),
			},
			spec: contract("frontend", "DEVELOPMENT", "0.9.0", "feature/login", "blanketops", "local"),
			labels: map[string]string{"environments.blanketops.dev/name": "env-development", "environments.blanketops.dev/type": "development"},
		},
		{
			name: "env-testing", ns: "blanketops-environments", phase: "ENVIRONMENT_PHASE_PENDING",
			conditions: []types.Condition{
				envCondition(false, "Build", "", "no BuildSuccess condition"),
				envCondition(false, "GitRepository", "", "no GitRepository condition"),
				envCondition(false, "Deployment", "", "no Deployment condition"),
			},
			spec: contract("auth-service", "TESTING", "0.3.0", "test/sso", "blanketops", "gcp"),
			labels: map[string]string{"environments.blanketops.dev/name": "env-testing", "environments.blanketops.dev/type": "testing"},
		},
	}
	for _, e := range envs {
		if err := add("environments", e.name, e.ns, e.phase, e.conditions, e.spec, e.labels); err != nil {
			return err
		}
	}

	// ── Builds ──
	builds := []struct {
		name, ns, phase string
		conditions      []types.Condition
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "build-payment-api", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "BuildSuccess", Status: "True", Reason: "BuildRunSucceeded", Message: "Build completed successfully", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/payment-api:2.1.0",
				"source": map[string]interface{}{
					"url":         "https://github.com/blanketops/payment-api.git",
					"revision":    "main",
					"contextDir":  "",
					"cloneSecret": "payment-api-git-ssh",
				},
				"strategy": map[string]interface{}{
					"name": "kaniko", "kind": "ClusterBuildStrategy",
				},
				"serviceAccount": map[string]interface{}{"name": "blanketops-sa"},
				"environmentRef": "env-production",
				"policy": map[string]interface{}{
					"allowedTriggers": []interface{}{map[string]interface{}{"type": "github.push"}},
					"retry":           map[string]interface{}{"onFailure": true, "maxAttempts": 3},
				},
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-production"},
		},
		{
			name: "build-orders-api", ns: "blanketops-environments", phase: "Running",
			conditions: []types.Condition{
				{Type: "BuildSuccess", Status: "False", Reason: "BuildRunning", Message: "Build is in progress", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/orders-api:1.4.0",
				"source": map[string]interface{}{
					"url":         "https://github.com/blanketops/orders-api.git",
					"revision":    "develop",
					"contextDir":  "",
					"cloneSecret": "orders-api-git-ssh",
				},
				"strategy": map[string]interface{}{
					"name": "buildah", "kind": "ClusterBuildStrategy",
				},
				"serviceAccount": map[string]interface{}{"name": "blanketops-sa"},
				"environmentRef": "env-staging",
				"policy": map[string]interface{}{
					"allowedTriggers": []interface{}{map[string]interface{}{"type": "github.push"}},
					"retry":           map[string]interface{}{"onFailure": true, "maxAttempts": 3},
				},
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging"},
		},
		{
			name: "build-frontend", ns: "blanketops-environments", phase: "Pending",
			conditions: []types.Condition{
				{Type: "BuildSuccess", Status: "False", Reason: "BuildPending", Message: "Build is not True", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/frontend:0.9.0",
				"source": map[string]interface{}{
					"url":         "https://github.com/blanketops/frontend.git",
					"revision":    "feature/login",
					"contextDir":  "",
					"cloneSecret": "frontend-git-ssh",
				},
				"strategy": map[string]interface{}{
					"name": "kaniko", "kind": "ClusterBuildStrategy",
				},
				"serviceAccount": map[string]interface{}{"name": "blanketops-sa"},
				"environmentRef": "env-development",
				"policy": map[string]interface{}{
					"allowedTriggers": []interface{}{map[string]interface{}{"type": "github.push"}},
					"retry":           map[string]interface{}{"onFailure": false},
				},
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-development"},
		},
	}
	for _, b := range builds {
		if err := add("builds", b.name, b.ns, b.phase, b.conditions, b.spec, b.labels); err != nil {
			return err
		}
	}

	// ── Packages ──
	pkgs := []struct {
		name, ns, phase string
		conditions      []types.Condition
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "pkg-cert-manager", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "Succeeded", Status: "True", Reason: "PackageInstalled", Message: "Package installed successfully", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/cert-manager:1.13.0",
				"source": map[string]interface{}{
					"url":         "https://github.com/blanketops/cert-manager.git",
					"revision":    "v1.13.0",
					"contextDir":  "deploy",
					"cloneSecret": "cert-manager-git-ssh",
				},
				"strategy": map[string]interface{}{"name": "kaniko", "kind": "ClusterBuildStrategy"},
				"serviceAccount": map[string]interface{}{"name": "blanketops-sa"},
				"environmentRef": "env-production",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-production"},
		},
		{
			name: "pkg-ingress-nginx", ns: "blanketops-environments", phase: "Running",
			conditions: []types.Condition{
				{Type: "Succeeded", Status: "False", Reason: "PackageReconciling", Message: "Package is being installed", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/ingress-nginx:1.9.0",
				"source": map[string]interface{}{
					"url":         "https://github.com/blanketops/ingress-nginx.git",
					"revision":    "main",
					"contextDir":  "",
					"cloneSecret": "ingress-git-ssh",
				},
				"strategy": map[string]interface{}{"name": "buildah", "kind": "ClusterBuildStrategy"},
				"serviceAccount": map[string]interface{}{"name": "blanketops-sa"},
				"environmentRef": "env-staging",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging"},
		},
	}
	for _, p := range pkgs {
		if err := add("packages", p.name, p.ns, p.phase, p.conditions, p.spec, p.labels); err != nil {
			return err
		}
	}

	// ── ServiceUnits ──
	sus := []struct {
		name, ns, phase string
		conditions      []types.Condition
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "svc-payment-worker", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "ServiceUnitReady", Status: "True", Reason: "ServiceUnitHealthy", Message: "Service unit is healthy", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/payment-worker:2.1.0",
				"replicas": 3,
				"source": map[string]interface{}{
					"url":         "https://github.com/blanketops/payment-worker.git",
					"cloneSecret": "payment-worker-git-ssh",
				},
				"strategy": map[string]interface{}{"name": "kaniko", "kind": "ClusterBuildStrategy"},
				"serviceAccount": map[string]interface{}{"name": "blanketops-sa"},
				"environmentRef": "env-production",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-production"},
		},
		{
			name: "svc-order-processor", ns: "blanketops-environments", phase: "Running",
			conditions: []types.Condition{
				{Type: "ServiceUnitReady", Status: "False", Reason: "ServiceUnitReconciling", Message: "Service unit is reconciling", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/order-processor:1.4.0",
				"replicas": 2,
				"source": map[string]interface{}{
					"url":         "https://github.com/blanketops/order-processor.git",
					"cloneSecret": "order-processor-git-ssh",
				},
				"strategy": map[string]interface{}{"name": "buildah", "kind": "ClusterBuildStrategy"},
				"serviceAccount": map[string]interface{}{"name": "blanketops-sa"},
				"environmentRef": "env-staging",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging"},
		},
	}
	for _, s := range sus {
		if err := add("serviceunits", s.name, s.ns, s.phase, s.conditions, s.spec, s.labels); err != nil {
			return err
		}
	}

	// ── Deployments ──
	deps := []struct {
		name, ns, phase string
		conditions      []types.Condition
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "deploy-payment-api", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "Ready", Status: "True", Reason: "DeploymentReady", Message: "Deployment rolled out", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/payment-api:2.1.0",
				"replicas": 2,
				"strategy": map[string]interface{}{"type": "RollingUpdate"},
				"environmentRef": "env-production",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-production"},
		},
		{
			name: "deploy-orders-api", ns: "blanketops-environments", phase: "Running",
			conditions: []types.Condition{
				{Type: "Ready", Status: "False", Reason: "DeploymentReconciling", Message: "Deployment is reconciling", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/orders-api:1.4.0",
				"replicas": 3,
				"strategy": map[string]interface{}{"type": "RollingUpdate"},
				"environmentRef": "env-staging",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging"},
		},
		{
			name: "deploy-frontend", ns: "blanketops-environments", phase: "Pending",
			conditions: []types.Condition{
				{Type: "Ready", Status: "False", Reason: "DeploymentPending", Message: "Deployment is pending", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"image": "ghcr.io/blanketops/frontend:0.9.0",
				"replicas": 1,
				"strategy": map[string]interface{}{"type": "Recreate"},
				"environmentRef": "env-development",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-development"},
		},
	}
	for _, d := range deps {
		if err := add("deployments", d.name, d.ns, d.phase, d.conditions, d.spec, d.labels); err != nil {
			return err
		}
	}

	// ── GitRepositories ──
	grs := []struct {
		name, ns, phase string
		conditions      []types.Condition
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "payment-api-repo", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "Ready", Status: "True", Reason: "RepositoryReady", Message: "Repository ready", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"url": "https://github.com/blanketops/payment-api.git",
				"revision": "main",
				"cloneSecret": "payment-api-git-ssh",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-production"},
		},
		{
			name: "orders-api-repo", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "Ready", Status: "True", Reason: "RepositoryReady", Message: "Repository ready", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"url": "https://github.com/blanketops/orders-api.git",
				"revision": "develop",
				"cloneSecret": "orders-api-git-ssh",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging"},
		},
	}
	for _, g := range grs {
		if err := add("gitrepositories", g.name, g.ns, g.phase, g.conditions, g.spec, g.labels); err != nil {
			return err
		}
	}

	// ── GitHubEvents ──
	ges := []struct {
		name, ns, phase string
		conditions      []types.Condition
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "event-push-payment", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "Ready", Status: "True", Reason: "EventProcessed", Message: "Push event processed", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"repository": "blanketops/payment-api",
				"event":      "push",
				"branch":     "main",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-production"},
		},
		{
			name: "event-pull-orders", ns: "blanketops-environments", phase: "Running",
			conditions: []types.Condition{
				{Type: "Ready", Status: "False", Reason: "EventPending", Message: "Pull request event pending", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"repository": "blanketops/orders-api",
				"event":      "pull_request",
				"branch":     "develop",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging"},
		},
	}
	for _, g := range ges {
		if err := add("githubevents", g.name, g.ns, g.phase, g.conditions, g.spec, g.labels); err != nil {
			return err
		}
	}

	// ── Routes ──
	routes := []struct {
		name, ns, phase string
		conditions      []types.Condition
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "route-payment-api", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "Ready", Status: "True", Reason: "RouteBound", Message: "Route bound", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"path": "/api/payments",
				"backend": "payment-api-service:8080",
				"tls": "edge",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-production"},
		},
		{
			name: "route-orders-api", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "Ready", Status: "True", Reason: "RouteBound", Message: "Route bound", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"path": "/api/orders",
				"backend": "orders-api-service:8080",
				"tls": "edge",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging"},
		},
	}
	for _, r := range routes {
		if err := add("routes", r.name, r.ns, r.phase, r.conditions, r.spec, r.labels); err != nil {
			return err
		}
	}

	// ── Domains ──
	doms := []struct {
		name, ns, phase string
		conditions      []types.Condition
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "payments-dev-host", ns: "blanketops-environments", phase: "Ready",
			conditions: []types.Condition{
				{Type: "Ready", Status: "True", Reason: "DomainReady", Message: "Domain ready", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"host": "payments.dev.blanketops.dev",
				"tlsStrategy": "platform",
				"ref": "env-production",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-production"},
		},
		{
			name: "orders-dev-host", ns: "blanketops-environments", phase: "Provisioning",
			conditions: []types.Condition{
				{Type: "Ready", Status: "False", Reason: "DomainProvisioning", Message: "Domain is being provisioned", LastTransitionTime: now},
			},
			spec: map[string]interface{}{
				"host": "orders.dev.blanketops.dev",
				"tlsStrategy": "custom",
				"ref": "env-staging",
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging"},
		},
	}
	for _, d := range doms {
		if err := add("domains", d.name, d.ns, d.phase, d.conditions, d.spec, d.labels); err != nil {
			return err
		}
	}

	slog.Info("mock store seeded", "items", len(m.data))
	return nil
}
