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

func (m *MockStore) seed() error {
	envs := []struct {
		name, ns, phase string
		spec            map[string]interface{}
		labels          map[string]string
	}{
		{
			name: "env-production", ns: "blanketops-environments", phase: "Ready",
			spec: map[string]interface{}{
				"contract": map[string]interface{}{
					"applicationName": "payment-api",
					"environmentType": "PRODUCTION",
					"version":         "2.1.0",
					"branch":          "main",
					"gitOwner":        "blanketops",
					"secretStore": map[string]interface{}{
						"provider": "vault",
						"vault": map[string]interface{}{
							"address":   "http://vault:8200",
							"path":      "secret",
							"role":      "payments-role",
							"mountPath": "kubernetes",
							"version":   "v2",
						},
					},
				},
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-production", "environments.blanketops.dev/type": "production"},
		},
		{
			name: "env-staging", ns: "blanketops-environments", phase: "Running",
			spec: map[string]interface{}{
				"contract": map[string]interface{}{
					"applicationName": "orders-api",
					"environmentType": "STAGING",
					"version":         "1.4.0",
					"branch":          "develop",
					"gitOwner":        "blanketops",
					"secretStore": map[string]interface{}{
						"provider": "aws",
					},
				},
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-staging", "environments.blanketops.dev/type": "staging"},
		},
		{
			name: "env-development", ns: "blanketops-environments", phase: "Pending",
			spec: map[string]interface{}{
				"contract": map[string]interface{}{
					"applicationName": "frontend",
					"environmentType": "DEVELOPMENT",
					"version":         "0.9.0",
					"branch":          "feature/login",
					"gitOwner":        "blanketops",
					"secretStore": map[string]interface{}{
						"provider": "local",
					},
				},
			},
			labels: map[string]string{"environments.blanketops.dev/name": "env-development", "environments.blanketops.dev/type": "development"},
		},
	}
	for _, e := range envs {
		specBytes, err := json.Marshal(e.spec)
		if err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339)
		item := types.CRStatus{
			Name:        e.name,
			Namespace:   e.ns,
			Kind:        "environments",
			Phase:       e.phase,
			Generation:  1,
			Conditions: []types.Condition{
				{Type: "Ready", Status: "True", Reason: "Reconciled", Message: "Environment reconciled", LastTransitionTime: now},
			},
			Events: []types.CREvent{
				{Type: "Normal", Reason: "Created", Message: "Environment created", Count: 1, FirstTime: now, LastTime: now},
			},
			Spec:        specBytes,
			Labels:      e.labels,
			Annotations: map[string]string{},
		}
		m.data[m.key("environments", e.ns, e.name)] = &item
	}
	return nil
}
