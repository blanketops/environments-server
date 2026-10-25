package store

import (
	"encoding/json"
	"time"

	"github.com/blanketops/environments-server/internal/types"
)

func agoISO(d time.Duration) string {
	return time.Now().UTC().Add(-d).Format(time.RFC3339)
}

func spec(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func readyConds(msg string) []types.Condition {
	return []types.Condition{{
		Type: "Ready", Status: "True", Reason: "ReconcileSuccess",
		Message: msg, LastTransitionTime: agoISO(10 * time.Minute),
	}}
}

func failedConds(msg string) []types.Condition {
	return []types.Condition{{
		Type: "Ready", Status: "False", Reason: "ReconcileError",
		Message: msg, LastTransitionTime: agoISO(time.Minute),
	}}
}

func pendingConds(msg string) []types.Condition {
	return []types.Condition{{
		Type: "Ready", Status: "False", Reason: "Initializing",
		Message: msg, LastTransitionTime: agoISO(10 * time.Second),
	}}
}

func ev(evType, reason, message string, count int) types.CREvent {
	return types.CREvent{
		Type: evType, Reason: reason, Message: message,
		Count:     count,
		FirstTime: agoISO(time.Hour),
		LastTime:  agoISO(30 * time.Second),
	}
}

// Seed loads representative data for all known kinds into the store.
func Seed(s *Store) {
	s.SeedKind("environments", []types.CRStatus{
		{
			Name: "environment-sample", Namespace: "blanketops-environments", Kind: "environments",
			Phase: "Ready", Generation: 3, Labels: map[string]string{"env": "dev"},
			Spec: spec(map[string]any{
				"envType":     "dev",
				"secretStore": map[string]any{"name": "blanketops-environments-aws", "kind": "ClusterSecretStore"},
			}),
			Conditions: []types.Condition{
				{Type: "SecretStoreReady", Status: "True", Reason: "Synced", Message: "AWS store reachable and synced", LastTransitionTime: agoISO(10 * time.Minute)},
				{Type: "ContractSatisfied", Status: "True", Reason: "Valid", Message: "All contract fields valid", LastTransitionTime: agoISO(10 * time.Minute)},
				{Type: "Ready", Status: "True", Reason: "ReconcileSuccess", Message: "Environment reconciled", LastTransitionTime: agoISO(10 * time.Minute)},
			},
			Events: []types.CREvent{ev("Normal", "ReconcileSuccess", "Environment reconciled", 12)},
		},
		{
			Name: "dev-sandbox-001", Namespace: "blanketops-environments", Kind: "environments",
			Phase: "Running", Generation: 1, Labels: map[string]string{"env": "dev"},
			Spec: spec(map[string]any{
				"envType":     "dev",
				"secretStore": map[string]any{"name": "blanketops-environments-gcp", "kind": "ClusterSecretStore"},
			}),
			Conditions: []types.Condition{
				{Type: "SecretStoreReady", Status: "True", Reason: "Synced", Message: "GCP store reachable", LastTransitionTime: agoISO(12 * time.Minute)},
				{Type: "ContractSatisfied", Status: "False", Reason: "Waiting", Message: "Waiting for build pipeline", LastTransitionTime: agoISO(12 * time.Minute)},
				{Type: "Ready", Status: "False", Reason: "Reconciling", Message: "Awaiting conditions", LastTransitionTime: agoISO(12 * time.Minute)},
			},
			Events: []types.CREvent{ev("Normal", "Reconciling", "Awaiting build pipeline", 3)},
		},
		{
			Name: "env-staging", Namespace: "blanketops-system", Kind: "environments",
			Phase: "Ready", Generation: 2, Labels: map[string]string{"env": "staging"},
			Spec:       spec(map[string]any{"envType": "staging", "secretStore": map[string]any{"name": "blanketops-environments-aws", "kind": "ClusterSecretStore"}}),
			Conditions: readyConds("Staging environment reconciled"),
			Events:     []types.CREvent{ev("Normal", "ReconcileSuccess", "Reconciled", 5)},
		},
	})

	s.SeedKind("builds", []types.CRStatus{
		{
			Name: "build-for-kaniko-sample", Namespace: "dev", Kind: "builds",
			Phase: "Running", Generation: 2,
			Spec: spec(map[string]any{
				"strategy":       map[string]any{"name": "kaniko", "kind": "ClusterBuildStrategy"},
				"source":         map[string]any{"url": "git@github.com:ntlaletsi70/blanketops-app.git", "revision": "main", "cloneSecret": "github-ssh-credentials"},
				"image":          "docker.io/nkanyezisolutions/for-kaniko-app:main",
				"serviceAccount": map[string]any{"name": "build-bot"},
			}),
			Conditions: []types.Condition{
				{Type: "ShipwrightBuildReady", Status: "True", Reason: "Emitted", Message: "Shipwright Build emitted", LastTransitionTime: agoISO(5 * time.Minute)},
				{Type: "BuildRunStarted", Status: "True", Reason: "Started", Message: "BuildRun in progress", LastTransitionTime: agoISO(5 * time.Minute)},
				{Type: "Ready", Status: "False", Reason: "Reconciling", Message: "Awaiting BuildRun completion", LastTransitionTime: agoISO(5 * time.Minute)},
			},
			Events: []types.CREvent{ev("Normal", "BuildStarted", "Build initiated", 1), ev("Normal", "BuildRunStarted", "BuildRun in progress", 1)},
		},
		{
			Name: "build-api", Namespace: "blanketops-system", Kind: "builds",
			Phase: "Ready", Generation: 4,
			Spec:       spec(map[string]any{"strategy": map[string]any{"name": "kaniko"}, "image": "registry.blanketops.dev/api:latest"}),
			Conditions: readyConds("Image pushed"),
			Events:     []types.CREvent{ev("Normal", "BuildComplete", "Image pushed to registry.blanketops.dev/api:latest", 1)},
		},
		{
			Name: "build-gateway", Namespace: "blanketops-system", Kind: "builds",
			Phase: "Failed", Generation: 3,
			Spec:       spec(map[string]any{"strategy": map[string]any{"name": "kaniko"}, "image": "registry.blanketops.dev/gateway:latest"}),
			Conditions: failedConds("kaniko: COPY failed: file not found in build context"),
			Events:     []types.CREvent{ev("Warning", "BuildFailed", "kaniko: COPY failed: file not found in build context or URL", 3)},
		},
	})

	s.SeedKind("packages", []types.CRStatus{
		{
			Name: "package-for-kaniko-sample", Namespace: "blanketops-environments", Kind: "packages",
			Phase: "Running", Generation: 1,
			Spec: spec(map[string]any{
				"name":        "for-kaniko-app",
				"version":     "v1.0.0",
				"packageRepo": "ntlaletsi70/for-kaniko-app-packages.git",
				"stateRepo":   map[string]any{"url": "ntlaletsi70/for-kaniko-app-state.git", "strategy": "kustomization", "path": "./clusters/dev"},
			}),
			Conditions: []types.Condition{
				{Type: "ImageAvailable", Status: "False", Reason: "Waiting", Message: "Awaiting Build completion", LastTransitionTime: agoISO(5 * time.Minute)},
				{Type: "PackagePublished", Status: "False", Reason: "Waiting", Message: "Awaiting image push", LastTransitionTime: agoISO(5 * time.Minute)},
				{Type: "Ready", Status: "False", Reason: "Reconciling", Message: "Awaiting conditions", LastTransitionTime: agoISO(5 * time.Minute)},
			},
			Events: []types.CREvent{},
		},
		{
			Name: "pkg-api-v1", Namespace: "blanketops-system", Kind: "packages",
			Phase: "Ready", Generation: 2,
			Spec:       spec(map[string]any{"strategy": "kustomize", "version": "v1.4.2"}),
			Conditions: readyConds("State repo synced"),
			Events:     []types.CREvent{},
		},
	})

	s.SeedKind("serviceunits", []types.CRStatus{
		{
			Name: "serviceunit-for-kaniko-web-sample", Namespace: "dev", Kind: "serviceunits",
			Phase: "Running", Generation: 1,
			Spec: spec(map[string]any{
				"type": "static", "image": "docker.io/nkanyezisolutions/for-kaniko-app:master",
				"containerPort": 8080, "size": 2, "appType": "web", "stackType": "nodejs",
			}),
			Conditions: []types.Condition{
				{Type: "PackageReady", Status: "False", Reason: "Waiting", Message: "Awaiting Package reconciliation", LastTransitionTime: agoISO(5 * time.Minute)},
				{Type: "Ready", Status: "False", Reason: "Reconciling", Message: "Awaiting conditions", LastTransitionTime: agoISO(5 * time.Minute)},
			},
			Events: []types.CREvent{},
		},
		{
			Name: "su-api", Namespace: "blanketops-system", Kind: "serviceunits",
			Phase: "Ready", Generation: 3,
			Spec:       spec(map[string]any{"type": "build", "buildRef": map[string]any{"name": "build-api"}, "imageRef": "registry.blanketops.dev/api:latest"}),
			Conditions: readyConds("Serving traffic"),
			Events:     []types.CREvent{},
		},
		{
			Name: "su-gateway", Namespace: "blanketops-system", Kind: "serviceunits",
			Phase: "Failed", Generation: 2,
			Spec:       spec(map[string]any{"type": "build", "buildRef": map[string]any{"name": "build-gateway"}}),
			Conditions: failedConds("Image build failed — cannot pull"),
			Events:     []types.CREvent{ev("Warning", "ImagePullFailed", "registry.blanketops.dev/gateway:latest not available", 4)},
		},
	})

	s.SeedKind("deployments", []types.CRStatus{
		{
			Name: "deployment-for-kaniko-sample", Namespace: "dev", Kind: "deployments",
			Phase: "Ready", Generation: 2,
			Spec: spec(map[string]any{
				"serviceUnits": []string{"for-kaniko-app-api", "for-kaniko-app-worker"},
				"strategy":     "Rolling", "runtime": "kubernetes.io/container-runtime",
				"manifestsRepo": map[string]any{"path": "./overlays/dev"},
			}),
			Conditions: readyConds("Deployment reconciled"),
			Events:     []types.CREvent{},
		},
		{
			Name: "deploy-production", Namespace: "blanketops-system", Kind: "deployments",
			Phase: "Ready", Generation: 5,
			Spec:       spec(map[string]any{"serviceUnits": []string{"su-api", "su-frontend"}, "strategy": "Rolling"}),
			Conditions: readyConds("All service units healthy"),
			Events:     []types.CREvent{},
		},
		{
			Name: "deploy-staging", Namespace: "blanketops-system", Kind: "deployments",
			Phase: "Pending", Generation: 1,
			Spec:       spec(map[string]any{"serviceUnits": []string{"su-api"}, "strategy": "Rolling"}),
			Conditions: pendingConds("su-gateway unhealthy"),
			Events:     []types.CREvent{ev("Warning", "ServiceUnitNotReady", "Waiting for su-gateway", 2)},
		},
	})

	s.SeedKind("gitrepositories", []types.CRStatus{
		{
			Name: "gitrepository-for-kaniko-sample", Namespace: "blanketops-environments", Kind: "gitrepositories",
			Phase: "Ready", Generation: 1,
			Spec: spec(map[string]any{
				"provider":   "github",
				"hookUrl":    "https://elimination-propecia-meter-strips.trycloudflare.com/",
				"repository": map[string]any{"owner": "ntlaletsi70", "name": "blanketops-app"},
				"webhooks":   map[string]any{"events": []string{"push", "pull_request"}},
			}),
			Conditions: readyConds("GitRepository synced and webhook active"),
			Events:     []types.CREvent{},
		},
		{
			Name: "repo-api", Namespace: "blanketops-system", Kind: "gitrepositories",
			Phase: "Ready", Generation: 1,
			Spec:       spec(map[string]any{"url": "git@github.com:blanketops/api.git", "branch": "main"}),
			Conditions: readyConds("Synced at abc1234"),
			Events:     []types.CREvent{},
		},
	})

	s.SeedKind("githubevents", []types.CRStatus{
		{
			Name: "githubevent-for-kaniko-sample", Namespace: "dev", Kind: "githubevents",
			Phase: "Ready", Generation: 1,
			Spec: spec(map[string]any{
				"repository": "ntlaletsi70/blanketops-app",
				"eventType":  "push", "ref": "refs/heads/main",
			}),
			Conditions: readyConds("Event processed"),
			Events:     []types.CREvent{},
		},
		{
			Name: "ghe-api-push", Namespace: "blanketops-system", Kind: "githubevents",
			Phase: "Ready", Generation: 1,
			Spec:       spec(map[string]any{"events": []string{"push"}, "filters": map[string]any{"branches": []string{"main"}}}),
			Conditions: readyConds("Webhook registered"),
			Events:     []types.CREvent{},
		},
	})

	s.SeedKind("routes", []types.CRStatus{
		{
			Name: "shopping-app-route", Namespace: "tenant-a", Kind: "routes",
			Phase: "Ready", Generation: 2,
			Spec:       spec(map[string]any{"host": "api.dev.domain.co.za", "path": "/", "tlsEnabled": true, "runtime": "kubernetes.io/container-runtime"}),
			Conditions: readyConds("Route serving"),
			Events:     []types.CREvent{},
		},
		{
			Name: "route-api-production", Namespace: "blanketops-system", Kind: "routes",
			Phase: "Ready", Generation: 2,
			Spec:       spec(map[string]any{"host": "api.dev.blanketops.dev", "path": "/api/v1", "tlsEnabled": true, "runtime": "knative"}),
			Conditions: readyConds("Route active"),
			Events:     []types.CREvent{},
		},
		{
			Name: "route-gateway", Namespace: "blanketops-system", Kind: "routes",
			Phase: "Failed", Generation: 2,
			Spec:       spec(map[string]any{"host": "gw.dev.blanketops.dev", "runtime": "knative"}),
			Conditions: failedConds("ServiceUnit su-gateway is not healthy"),
			Events:     []types.CREvent{ev("Warning", "BackendNotReady", "Upstream su-gateway returning 503", 6)},
		},
	})

	s.SeedKind("domains", []types.CRStatus{
		{
			Name: "shopping-app-domain", Namespace: "tenant-a", Kind: "domains",
			Phase: "Running", Generation: 2,
			Spec: spec(map[string]any{
				"host": "api.dev.domain.co.za", "tlsStrategy": "platform",
				"routeRef": map[string]any{"name": "shopping-app-route"},
			}),
			Conditions: []types.Condition{
				{Type: "DomainClaimReady", Status: "True", Reason: "Reconciled", Message: "DomainClaim reconciled", LastTransitionTime: agoISO(48 * time.Hour)},
				{Type: "CertificateReady", Status: "False", Reason: "Provisioning", Message: "Wildcard cert provisioning", LastTransitionTime: agoISO(48 * time.Hour)},
				{Type: "Ready", Status: "False", Reason: "Reconciling", Message: "Awaiting cert", LastTransitionTime: agoISO(48 * time.Hour)},
			},
			Events: []types.CREvent{},
		},
		{
			Name: "domain-blanketops-dev", Namespace: "blanketops-system", Kind: "domains",
			Phase: "Ready", Generation: 2,
			Spec:       spec(map[string]any{"host": "blanketops.dev", "tlsStrategy": "DNS01"}),
			Conditions: readyConds("Wildcard cert issued for *.dev.blanketops.dev"),
			Events:     []types.CREvent{},
		},
	})

	s.SeedKind("supplychains", []types.CRStatus{
		{
			Name: "for-kaniko-app", Namespace: "default", Kind: "supplychains",
			Phase: "Ready", Generation: 1,
			Spec: spec(map[string]any{
				"repository":  "ntlaletsi70/for-kaniko-app",
				"webhookHost": "pop-os.tailf8145.ts.net",
				"steps":       map[string]any{"sign": true, "attest": true, "buildpacks": true},
			}),
			Conditions: []types.Condition{
				{Type: "WebhookReady", Status: "True", Reason: "Registered", Message: "GitHubWebhook registered and active", LastTransitionTime: agoISO(72 * time.Hour)},
				{Type: "PipelineReady", Status: "True", Reason: "Reconciled", Message: "Tekton Pipeline reconciled", LastTransitionTime: agoISO(72 * time.Hour)},
				{Type: "Ready", Status: "True", Reason: "ReconcileSuccess", Message: "SupplyChain reconciled", LastTransitionTime: agoISO(72 * time.Hour)},
			},
			Events: []types.CREvent{ev("Normal", "PipelineRunStarted", "Tekton PipelineRun triggered for push event", 1)},
		},
	})

	s.SeedKind("githubwebhooks", []types.CRStatus{
		{
			Name: "for-kaniko-app-webhook", Namespace: "default", Kind: "githubwebhooks",
			Phase: "Ready", Generation: 1,
			Spec: spec(map[string]any{
				"repository":     "ntlaletsi70/for-kaniko-app",
				"supplyChainRef": map[string]any{"name": "for-kaniko-app"},
				"hookURL":        "https://pop-os.tailf8145.ts.net",
				"events":         []string{"push"},
			}),
			Conditions: readyConds("GitHubWebhook active"),
			Events:     []types.CREvent{},
		},
	})
}
