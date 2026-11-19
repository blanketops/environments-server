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
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/blanketops/environments-server/pkg/helpers"
	"github.com/blanketops/environments-server/pkg/store"
)

func SetupRoutes(s store.Store) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", HandleHealth(s))
	mux.HandleFunc("GET /api/v1/{kind}/{ns}/{name}/events", HandleEvents(s))
	mux.HandleFunc("GET /api/v1/{kind}/{ns}/{name}/debug", HandleDebug(s))
	mux.HandleFunc("GET /api/v1/{kind}/{ns}/{name}", HandleGet(s))
	mux.HandleFunc("PATCH /api/v1/{kind}/{ns}/{name}", HandlePatch(s))
	mux.HandleFunc("DELETE /api/v1/{kind}/{ns}/{name}", HandleDelete(s))
	mux.HandleFunc("GET /api/v1/{kind}", HandleList(s))
	mux.HandleFunc("POST /api/v1/{kind}", HandleCreate(s))
	mux.HandleFunc("GET /api/v1/pods/{namespace}/{pod}/logs", HandleLogs(s))
	mux.HandleFunc("GET /api/v1/pods", HandleListPods(s))

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

func HandleHealth(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Real connectivity check: try listing environments.
		// If the store can't reach the backend (k8s or mock), report connected:false.
		_, err := s.List("environments", "")
		connected := err == nil

		w.Header().Set("Content-Type", "application/json")
		if !connected {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"degraded","connected":false,"store":"kubernetes"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","connected":true,"store":"kubernetes"}`))
	}
}

func HandleEvents(s store.Store) http.HandlerFunc {
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

func HandleListPods(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ns := r.URL.Query().Get("namespace")
		if ns == "" {
			ns = "blanketops-environments"
		}

		pods, err := s.ListPods(ns)
		if err != nil {
			if se, ok := err.(*store.StoreError); ok {
				helpers.WriteErr(w, se.HTTPStatus(), se.Message, se.Reason)
			} else {
				helpers.WriteErr(w, 500, err.Error(), "InternalServerError")
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(pods); err != nil {
			slog.Error("encode pods", "err", err)
		}
	}
}

func HandleLogs(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ns := r.PathValue("namespace")
		pod := r.PathValue("pod")
		container := r.URL.Query().Get("container")
		if container == "" {
			container = "manager"
		}
		tailLines := int64(200)
		if v := r.URL.Query().Get("tailLines"); v != "" {
			if n, err := fmt.Sscanf(v, "%d", &tailLines); err != nil || n != 1 {
				tailLines = 200
			}
		}

		logs, err := s.Logs(pod, ns, container, tailLines)
		if err != nil {
			if se, ok := err.(*store.StoreError); ok {
				helpers.WriteErr(w, se.HTTPStatus(), se.Message, se.Reason)
			} else {
				helpers.WriteErr(w, 500, err.Error(), "InternalServerError")
			}
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(logs))
	}
}

func HandleDebug(s store.Store) http.HandlerFunc {
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