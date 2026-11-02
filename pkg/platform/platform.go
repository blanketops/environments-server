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
package platform

import (
	"github.com/blanketops/environments-server/pkg/types"
)

// PlatformDependencies represents the platform dependencies from the CLI registry
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

func IsPlatformDependency(kind string) bool {
	platformDeps := map[string]bool{
		"carvel": true, "argoevents": true, "tekton-pipelines": true,
		"tekton-dashboard": true, "shipwright": true, "crossplane": true,
		"externalsecrets": true, "buildstrategies": true,
		"flux": true, "knative": true, "kourier": true,
	}
	return platformDeps[kind]
}

func FilterPlatformDependencies(kind string) []types.CRStatus {
	for _, dep := range PlatformDependencies {
		if dep.Kind == kind {
			return []types.CRStatus{dep}
		}
	}
	return nil
}
