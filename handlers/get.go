package handlers

import (
	"net/http"

	"github.com/blanketops/environments-server/pkg/helpers"
	"github.com/blanketops/environments-server/pkg/store"
)

func HandleGet(s *store.K8sStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.PathValue("ns")
		name := r.PathValue("name")

		item, err := s.Get(kind, ns, name)
		if err != nil {
			if se, ok := err.(*store.StoreError); ok {
				helpers.WriteErr(w, se.HTTPStatus(), se.Message, se.Reason)
			} else {
				helpers.WriteErr(w, 500, err.Error(), "InternalServerError")
			}
			return
		}

		// Also fetch Kubernetes events and merge them
		events, err := s.GetEvents(kind, ns, name)
		if err == nil && len(events) > 0 {
			item.Events = events
		}

		helpers.WriteJSON(w, 200, item)
	}
}