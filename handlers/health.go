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
	"log/slog"
	"net/http"

	"github.com/blanketops/environments-server/pkg/helpers"
	"github.com/blanketops/environments-server/pkg/store"
)

func SetupRoutes(s *store.K8sStore) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", HandleHealth())
	mux.HandleFunc("GET /api/v1/{kind}/{ns}/{name}/events", HandleEvents(s))
	mux.HandleFunc("GET /api/v1/{kind}/{ns}/{name}/debug", HandleDebug(s))
	mux.HandleFunc("GET /api/v1/{kind}/{ns}/{name}", HandleGet(s))
	mux.HandleFunc("PATCH /api/v1/{kind}/{ns}/{name}", HandlePatch(s))
	mux.HandleFunc("DELETE /api/v1/{kind}/{ns}/{name}", HandleDelete(s))
	mux.HandleFunc("GET /api/v1/{kind}", HandleList(s))
	mux.HandleFunc("POST /api/v1/{kind}", HandleCreate(s))

	return mux
}

func Cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func HandleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","connected":true,"store":"kubernetes"}`))
	}
}

func HandleEvents(s *store.K8sStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.PathValue("ns")
		name := r.PathValue("name")

		slog.Debug("HandleEvents: called", "kind", kind, "namespace", ns, "name", name)
		events, err := s.GetEvents(kind, ns, name)
		slog.Debug("HandleEvents: GetEvents returned", "count", len(events), "err", err)
		if err != nil {
			if se, ok := err.(*store.StoreError); ok {
				helpers.WriteErr(w, se.HTTPStatus(), se.Message, se.Reason)
			} else {
				helpers.WriteErr(w, 500, err.Error(), "InternalServerError")
			}
			return
		}
		helpers.WriteJSON(w, 200, events)
	}
}

func HandleDebug(s *store.K8sStore) http.HandlerFunc {
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
		debugInfo := store.BuildDebugInfo(item)
		helpers.WriteJSON(w, 200, debugInfo)
	}
}