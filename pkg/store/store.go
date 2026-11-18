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

import "github.com/blanketops/environments-server/pkg/types"

// Store is the read/write interface for environment CRs.
// K8sStore (live cluster) and MockStore (offline/demo) both satisfy it.
// Handlers depend only on this interface so the server can run without a cluster.
type Store interface {
	List(kind, namespace string) ([]types.CRStatus, error)
	Get(kind, namespace, name string) (types.CRStatus, error)
	Create(kind, defaultNS string, body types.CreateBody) (types.CRStatus, error)
	Patch(kind, namespace, name string, body types.PatchBody) (types.CRStatus, error)
	Delete(kind, namespace, name string) (types.DeleteResponse, error)
	GetEvents(kind, namespace, name string) ([]types.CREvent, error)
}
