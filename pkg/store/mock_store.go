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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/blanketops/environments-server/pkg/types"
)

// MockStore is an in-memory, thread-safe store. It starts empty and is
// populated only through API calls (create/patch). No mock data is seeded.
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
	return nil, nil
}

// GetLogs returns container logs via the Kubernetes API.
func (m *MockStore) Logs(pod, namespace, container string, tailLines int64) (string, error) {
	return "", fmt.Errorf("mock store does not support log streaming; use a live cluster")
}

// ListPods returns pod names in a namespace via the Kubernetes API.
func (m *MockStore) ListPods(namespace string) ([]string, error) {
	return nil, fmt.Errorf("mock store does not support pod listing; use a live cluster")
}

func (m *MockStore) seed() error {
	return nil
}
