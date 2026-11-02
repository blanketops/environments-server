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

	"github.com/blanketops/environments-server/pkg/helpers"
	"github.com/blanketops/environments-server/pkg/platform"
	"github.com/blanketops/environments-server/pkg/store"
)

func HandleDelete(s *store.K8sStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.PathValue("ns")
		name := r.PathValue("name")

		// Platform dependencies (not K8s CRs) - cannot delete
		if platform.IsPlatformDependency(kind) {
			helpers.WriteErr(w, 400, fmt.Sprintf("delete not supported for platform dependency: %s", kind), "BadRequest")
			return
		}

		resp, err := s.Delete(kind, ns, name)
		if err != nil {
			if se, ok := err.(*store.StoreError); ok {
				helpers.WriteErr(w, se.HTTPStatus(), se.Message, se.Reason)
			} else {
				helpers.WriteErr(w, 500, err.Error(), "InternalServerError")
			}
			return
		}
		helpers.WriteJSON(w, 200, resp)
	}
}