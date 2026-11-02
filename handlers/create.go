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
package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/blanketops/environments-server/pkg/helpers"
	"github.com/blanketops/environments-server/pkg/platform"
	"github.com/blanketops/environments-server/pkg/store"
	"github.com/blanketops/environments-server/pkg/types"
)

func HandleCreate(s *store.K8sStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.URL.Query().Get("namespace")
		if ns == "" {
			ns = "default"
		}
		body, err := helpers.Decode[types.CreateBody](r)
		if err != nil {
			helpers.WriteErr(w, 400, err.Error(), "BadRequest")
			return
		}
		name := body.Metadata.Name
		if name == "" {
			name = fmt.Sprintf("%s-%d", kind, time.Now().UnixMilli())
		}

		// Platform dependencies (not K8s CRs) - cannot create
		if platform.IsPlatformDependency(kind) {
			helpers.WriteErr(w, 400, fmt.Sprintf("create not supported for platform dependency: %s", kind), "BadRequest")
			return
		}

		item, err := s.Create(kind, ns, body)
		if err != nil {
			if se, ok := err.(*store.StoreError); ok {
				helpers.WriteErr(w, se.HTTPStatus(), se.Message, se.Reason)
			} else {
				helpers.WriteErr(w, 500, err.Error(), "InternalServerError")
			}
			return
		}
		helpers.WriteJSON(w, 201, item)
	}
}