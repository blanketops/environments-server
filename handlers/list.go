package handlers

import (
	"net/http"

	"github.com/blanketops/environments-server/pkg/helpers"
	"github.com/blanketops/environments-server/pkg/platform"
	"github.com/blanketops/environments-server/pkg/store"
	"github.com/blanketops/environments-server/pkg/types"
)

func HandleList(s *store.K8sStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.URL.Query().Get("namespace")

		// Platform dependencies (not K8s CRs) - return static list
		if platform.IsPlatformDependency(kind) {
			items := platform.FilterPlatformDependencies(kind)
			if items != nil {
				helpers.WriteJSON(w, 200, types.ListResponse{Items: items, Total: len(items)})
				return
			}
		}

		items, err := s.List(kind, ns)
		if err != nil {
			helpers.WriteErr(w, 500, err.Error(), "InternalServerError")
			return
		}
		helpers.WriteJSON(w, 200, types.ListResponse{Items: items, Total: len(items)})
	}
}