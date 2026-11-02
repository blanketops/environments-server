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
package helpers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/blanketops/environments-server/pkg/types"
)

var PlatformDependencies = []types.CRStatus{
	{Name: "carvel", Namespace: "kapp-controller", Kind: "carvel", Phase: "Ready", Labels: map[string]string{"provider": "carvel"}, Annotations: map[string]string{"description": "Carvel Kapp Controller — packaging and lifecycle management"}},
	{Name: "argoevents", Namespace: "argo-events", Kind: "argoevents", Phase: "Ready", Labels: map[string]string{"provider": "argoevents"}, Annotations: map[string]string{"description": "Argo Events — event-driven pipelines"}},
	{Name: "tekton-pipelines", Namespace: "tekton-pipelines", Kind: "tekton-pipelines", Phase: "Ready", Labels: map[string]string{"provider": "tekton"}, Annotations: map[string]string{"description": "Tekton Pipelines — CI/CD execution engine"}},
	{Name: "tekton-dashboard", Namespace: "tekton-pipelines", Kind: "tekton-dashboard", Phase: "Ready", Labels: map[string]string{"provider": "tekton"}, Annotations: map[string]string{"description": "Tekton Dashboard — pipeline UI"}},
	{Name: "shipwright", Namespace: "shipwright-build", Kind: "shipwright", Phase: "Ready", Labels: map[string]string{"provider": "shipwright"}, Annotations: map[string]string{"description": "Shipwright Build — Kubernetes-native image builds"}},
	{Name: "crossplane", Namespace: "crossplane-system", Kind: "crossplane", Phase: "Ready", Labels: map[string]string{"provider": "crossplane"}, Annotations: map[string]string{"description": "Crossplane — infrastructure orchestration"}},
	{Name: "externalsecrets", Namespace: "external-secrets", Kind: "externalsecrets", Phase: "Ready", Labels: map[string]string{"provider": "externalsecrets"}, Annotations: map[string]string{"description": "External Secrets Operator — secure secret integration"}},
	{Name: "buildstrategies", Namespace: "shipwright-build", Kind: "buildstrategies", Phase: "Ready", Labels: map[string]string{"provider": "shipwright"}, Annotations: map[string]string{"description": "Shipwright build strategies (buildpacks, kaniko, buildah)"}},
	{Name: "flux", Namespace: "flux-system", Kind: "flux", Phase: "Ready", Labels: map[string]string{"provider": "flux"}, Annotations: map[string]string{"description": "Flux UI + config"}},
	{Name: "knative", Namespace: "knative-serving", Kind: "knative", Phase: "Ready", Labels: map[string]string{"provider": "knative"}, Annotations: map[string]string{"description": "Knative Serving — serverless workload runtime"}},
	{Name: "kourier", Namespace: "kourier-system", Kind: "kourier", Phase: "Ready", Labels: map[string]string{"provider": "kourier"}, Annotations: map[string]string{"description": "Kourier — Knative ingress/networking layer"}},
}

func isPlatformDependency(kind string) bool {
	platformDeps := map[string]bool{
		"carvel": true, "argoevents": true, "tekton-pipelines": true,
		"tekton-dashboard": true, "shipwright": true, "crossplane": true,
		"externalsecrets": true, "buildstrategies": true,
		"flux": true, "knative": true, "kourier": true,
	}
	return platformDeps[kind]
}

func filterPlatformDependencies(kind string) []types.CRStatus {
	for _, dep := range PlatformDependencies {
		if dep.Kind == kind {
			return []types.CRStatus{dep}
		}
	}
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode", "err", err)
	}
}

// WriteErr writes an error response (exported version)
func WriteErr(w http.ResponseWriter, status int, msg, reason string) {
	WriteJSON(w, status, types.APIError{Message: msg, Reason: reason, Code: status})
}

func Decode[T any](r *http.Request) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("invalid request body: %w", err)
	}
	return v, nil
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.DefaultServeMux.ServeHTTP(w, r)
	})
}
