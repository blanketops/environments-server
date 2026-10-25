package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	environmentsv1alpha1 "github.com/ntlaletsi70/blanketops-environments-api/api/environments/v1alpha1"
	eventsv1alpha1 "github.com/ntlaletsi70/blanketops-environments-api/api/events/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/blanketops/environments-server/internal/types"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	// ── Scheme ─────────────────────────────────────────────────────────
	scheme := runtime.NewScheme()
	_ = environmentsv1alpha1.AddToScheme(scheme)
	_ = eventsv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// ── K8s client ─────────────────────────────────────────────────────
	k8sClient, err := newK8sClient(scheme)
	if err != nil {
		slog.Warn("k8s client unavailable", "err", err)
	}

	// ── Router ─────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handleHealth(k8sClient))
	mux.HandleFunc("GET /api/v1/{kind}/{ns}/{name}/events", handleEvents(k8sClient))
	mux.HandleFunc("GET /api/v1/{kind}/{ns}/{name}/debug", handleDebug(k8sClient))
	mux.HandleFunc("GET /api/v1/{kind}/{ns}/{name}", handleGet(k8sClient))
	mux.HandleFunc("PATCH /api/v1/{kind}/{ns}/{name}", handlePatch(k8sClient))
	mux.HandleFunc("DELETE /api/v1/{kind}/{ns}/{name}", handleDelete(k8sClient))
	mux.HandleFunc("GET /api/v1/{kind}", handleList(k8sClient))
	mux.HandleFunc("POST /api/v1/{kind}", handleCreate(k8sClient))

	// ── UI ─────────────────────────────────────────────────────────────
	// The ko image bundles the built UI as kodata; serve it from the same
	// origin as the API so the image runs without the nginx proxy.
	if dir := uiDir(); dir != "" {
		mux.Handle("GET /", uiHandler(dir))
		slog.Info("serving ui", "dir", dir)
	}

	slog.Info("blanketops server", "addr", ":"+port, "k8s", k8sClient != nil)
	if err := http.ListenAndServe(":"+port, cors(mux)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func newK8sClient(scheme *runtime.Scheme) (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("no kubeconfig: %w", err)
		}
	}
	return client.New(cfg, client.Options{Scheme: scheme})
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
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode", "err", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg, reason string) {
	writeJSON(w, status, types.APIError{Message: msg, Reason: reason, Code: status})
}

func decode[T any](r *http.Request) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("invalid request body: %w", err)
	}
	return v, nil
}

// ── Health ────────────────────────────────────────────────────────────────

func handleHealth(k8s client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if k8s == nil {
			writeJSON(w, 503, map[string]any{
				"status": "unavailable", "connected": false,
				"reason": "Kubernetes client not initialized",
			})
			return
		}
		var nsList corev1.NamespaceList
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := k8s.List(ctx, &nsList, client.Limit(1)); err != nil {
			writeJSON(w, 503, map[string]any{
				"status": "unavailable", "connected": false,
				"reason": err.Error(),
			})
			return
		}
		writeJSON(w, 200, map[string]any{
			"status": "ok", "connected": true, "store": "kubernetes",
		})
	}
}

// ── List ──────────────────────────────────────────────────────────────────

func handleList(k8s client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.URL.Query().Get("namespace")

		switch kind {
		case "builds":
			var list environmentsv1alpha1.BuildList
			opts := []client.ListOption{}
			if ns != "" {
				opts = append(opts, client.InNamespace(ns))
			}
			if err := k8s.List(r.Context(), &list, opts...); err != nil {
				writeErr(w, 500, err.Error(), "InternalServerError")
				return
			}
			items := make([]types.CRStatus, len(list.Items))
			for i, b := range list.Items {
				items[i] = buildToStatus(&b)
			}
			writeJSON(w, 200, types.ListResponse{Items: items, Total: len(items)})

		case "githubevents":
			var list eventsv1alpha1.GitHubEventList
			opts := []client.ListOption{}
			if ns != "" {
				opts = append(opts, client.InNamespace(ns))
			}
			if err := k8s.List(r.Context(), &list, opts...); err != nil {
				writeErr(w, 500, err.Error(), "InternalServerError")
				return
			}
			items := make([]types.CRStatus, len(list.Items))
			for i, e := range list.Items {
				items[i] = githubEventToStatus(&e)
			}
			writeJSON(w, 200, types.ListResponse{Items: items, Total: len(items)})

		default:
			writeErr(w, 400, fmt.Sprintf("unknown kind: %s", kind), "BadRequest")
		}
	}
}

// ── Get ───────────────────────────────────────────────────────────────────

func handleGet(k8s client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.PathValue("ns")
		name := r.PathValue("name")

		switch kind {
		case "builds":
			var build environmentsv1alpha1.Build
			if err := k8s.Get(r.Context(), client.ObjectKey{Namespace: ns, Name: name}, &build); err != nil {
				writeErr(w, 404, err.Error(), "NotFound")
				return
			}
			writeJSON(w, 200, buildToStatus(&build))
		case "githubevents":
			var event eventsv1alpha1.GitHubEvent
			if err := k8s.Get(r.Context(), client.ObjectKey{Namespace: ns, Name: name}, &event); err != nil {
				writeErr(w, 404, err.Error(), "NotFound")
				return
			}
			writeJSON(w, 200, githubEventToStatus(&event))
		default:
			writeErr(w, 400, fmt.Sprintf("unknown kind: %s", kind), "BadRequest")
		}
	}
}

// ── Create ────────────────────────────────────────────────────────────────

func handleCreate(k8s client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.URL.Query().Get("namespace")
		if ns == "" {
			ns = "default"
		}
		body, err := decode[types.CreateBody](r)
		if err != nil {
			writeErr(w, 400, err.Error(), "BadRequest")
			return
		}
		name := body.Metadata.Name
		if name == "" {
			name = fmt.Sprintf("%s-%d", kind, time.Now().UnixMilli())
		}

		switch kind {
		case "builds":
			build := &environmentsv1alpha1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name: name, Namespace: ns,
					Labels:      body.Metadata.Labels,
					Annotations: body.Metadata.Annotations,
				},
				Spec: environmentsv1alpha1.BuildSpec{
					Contract: runtime.RawExtension{Raw: body.Spec},
				},
			}
			if err := k8s.Create(r.Context(), build); err != nil {
				writeErr(w, 500, err.Error(), "InternalServerError")
				return
			}
			writeJSON(w, 201, buildToStatus(build))

		case "githubevents":
			event := &eventsv1alpha1.GitHubEvent{
				ObjectMeta: metav1.ObjectMeta{
					Name: name, Namespace: ns,
					Labels:      body.Metadata.Labels,
					Annotations: body.Metadata.Annotations,
				},
				Spec: eventsv1alpha1.GitHubEventSpec{
					Contract: runtime.RawExtension{Raw: body.Spec},
				},
			}
			if err := k8s.Create(r.Context(), event); err != nil {
				writeErr(w, 500, err.Error(), "InternalServerError")
				return
			}
			writeJSON(w, 201, githubEventToStatus(event))

		default:
			writeErr(w, 400, fmt.Sprintf("create not implemented for kind: %s", kind), "BadRequest")
		}
	}
}

// ── Patch ─────────────────────────────────────────────────────────────────

func handlePatch(k8s client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.PathValue("ns")
		name := r.PathValue("name")
		body, err := decode[types.PatchBody](r)
		if err != nil {
			writeErr(w, 400, err.Error(), "BadRequest")
			return
		}

		switch kind {
		case "builds":
			var build environmentsv1alpha1.Build
			if err := k8s.Get(r.Context(), client.ObjectKey{Namespace: ns, Name: name}, &build); err != nil {
				writeErr(w, 404, err.Error(), "NotFound")
				return
			}
			original := build.DeepCopy()
			if body.Spec != nil {
				build.Spec.Contract = runtime.RawExtension{Raw: body.Spec}
			}
			if body.Metadata != nil {
				mergeMeta(&build.ObjectMeta, body.Metadata)
			}
			if err := k8s.Patch(r.Context(), &build, client.MergeFrom(original)); err != nil {
				writeErr(w, 500, err.Error(), "InternalServerError")
				return
			}
			writeJSON(w, 200, buildToStatus(&build))

		case "githubevents":
			var event eventsv1alpha1.GitHubEvent
			if err := k8s.Get(r.Context(), client.ObjectKey{Namespace: ns, Name: name}, &event); err != nil {
				writeErr(w, 404, err.Error(), "NotFound")
				return
			}
			original := event.DeepCopy()
			if body.Spec != nil {
				event.Spec.Contract = runtime.RawExtension{Raw: body.Spec}
			}
			if body.Metadata != nil {
				mergeMeta(&event.ObjectMeta, body.Metadata)
			}
			if err := k8s.Patch(r.Context(), &event, client.MergeFrom(original)); err != nil {
				writeErr(w, 500, err.Error(), "InternalServerError")
				return
			}
			writeJSON(w, 200, githubEventToStatus(&event))

		default:
			writeErr(w, 400, fmt.Sprintf("patch not implemented for kind: %s", kind), "BadRequest")
		}
	}
}

// ── Delete ────────────────────────────────────────────────────────────────

func handleDelete(k8s client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		ns := r.PathValue("ns")
		name := r.PathValue("name")

		switch kind {
		case "builds":
			build := &environmentsv1alpha1.Build{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			}
			if err := k8s.Delete(r.Context(), build); err != nil {
				writeErr(w, 500, err.Error(), "InternalServerError")
				return
			}
			writeJSON(w, 200, types.DeleteResponse{Name: name, Namespace: ns, Kind: kind, Deleted: true})
		case "githubevents":
			event := &eventsv1alpha1.GitHubEvent{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			}
			if err := k8s.Delete(r.Context(), event); err != nil {
				writeErr(w, 500, err.Error(), "InternalServerError")
				return
			}
			writeJSON(w, 200, types.DeleteResponse{Name: name, Namespace: ns, Kind: kind, Deleted: true})
		default:
			writeErr(w, 400, fmt.Sprintf("delete not implemented for kind: %s", kind), "BadRequest")
		}
	}
}

// ── Sub-resources ─────────────────────────────────────────────────────────

func handleEvents(k8s client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, []types.CREvent{})
	}
}

func handleDebug(k8s client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, types.DebugInfo{
			Name: r.PathValue("name"), Namespace: r.PathValue("ns"), Kind: r.PathValue("kind"),
			QueueDepth: 0,
		})
	}
}

// ── Status converters ─────────────────────────────────────────────────────

func buildToStatus(b *environmentsv1alpha1.Build) types.CRStatus {
	conds := make([]types.Condition, len(b.Status.Conditions))
	for i, c := range b.Status.Conditions {
		conds[i] = types.Condition{
			Type: c.Type, Status: string(c.Status), Reason: c.Reason,
			Message: c.Message, LastTransitionTime: c.LastTransitionTime.Format(time.RFC3339),
		}
	}
	annotations := b.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}
	if len(b.Status.Contract.Raw) > 0 {
		var cs map[string]interface{}
		if err := json.Unmarshal(b.Status.Contract.Raw, &cs); err == nil {
			if ref, ok := cs["ExecutionRef"].(string); ok && ref != "" {
				annotations["build.blanketops.dev/executionRef"] = ref
			}
			if hash, ok := cs["BuildHash"].(string); ok && hash != "" {
				annotations["build.blanketops.dev/buildHash"] = hash
			}
			if msg, ok := cs["Message"].(string); ok && msg != "" {
				annotations["build.blanketops.dev/message"] = msg
			}
			if success, ok := cs["Success"].(bool); ok {
				annotations["build.blanketops.dev/success"] = fmt.Sprintf("%v", success)
			}
		}
	}
	return types.CRStatus{
		Name: b.Name, Namespace: b.Namespace, Kind: "Build",
		Phase: inferBuildPhase(conds), Generation: b.Generation,
		Labels: b.Labels, Annotations: annotations, Conditions: conds,
		Spec: b.Spec.Contract.Raw,
	}
}

func githubEventToStatus(e *eventsv1alpha1.GitHubEvent) types.CRStatus {
	conds := make([]types.Condition, len(e.Status.Conditions))
	for i, c := range e.Status.Conditions {
		conds[i] = types.Condition{
			Type: c.Type, Status: string(c.Status), Reason: c.Reason,
			Message: c.Message, LastTransitionTime: c.LastTransitionTime.Format(time.RFC3339),
		}
	}
	return types.CRStatus{
		Name: e.Name, Namespace: e.Namespace, Kind: "GitHubEvent",
		Phase: inferGitHubEventPhase(conds), Generation: e.Generation,
		Labels: e.Labels, Annotations: e.Annotations, Conditions: conds,
		Spec: e.Spec.Contract.Raw,
	}
}

func inferBuildPhase(conds []types.Condition) types.Phase {
	var resolved, cached, prereqs, ready, started, succeeded, failed bool
	for _, c := range conds {
		switch c.Type {
		case "BuildResolved":
			resolved = c.Status == "True"
		case "BuildCached":
			cached = c.Status == "True"
		case "BuildPrerequisitesCreated":
			prereqs = c.Status == "True"
		case "BuildReady":
			ready = c.Status == "True"
		case "BuildStart":
			started = c.Status == "True"
		case "BuildSuccess":
			succeeded = c.Status == "True"
		}
		if c.Status == "False" {
			switch c.Reason {
			case "BuildFailed", "ResolveFailed", "PrerequisitesFailed", "CacheFailed":
				failed = true
			}
		}
	}
	if failed {
		return types.PhaseFailed
	}
	if succeeded {
		return types.PhaseReady
	}
	if started {
		return types.PhaseRunning
	}
	if ready || prereqs || cached || resolved {
		return types.PhasePending
	}
	return types.PhaseUnknown
}

func inferGitHubEventPhase(conds []types.Condition) types.Phase {
	var resolved, cached, prereqs, organized, accepted bool
	var failed bool
	for _, c := range conds {
		switch c.Type {
		case "GitHubEventResolved":
			resolved = c.Status == "True"
		case "GitHubEventCached":
			cached = c.Status == "True"
		case "GitHubEventPrerequisitesCreated":
			prereqs = c.Status == "True"
		case "GitHubEventOrganized":
			organized = c.Status == "True"
		case "GitHubEventAccepted":
			accepted = c.Status == "True"
		}
		if c.Status == "False" {
			switch c.Reason {
			case "GitHubEventRejected", "MatchFailed", "TriggerFailed":
				failed = true
			}
		}
	}
	if failed {
		return types.PhaseFailed
	}
	if accepted {
		return types.PhaseReady
	}
	if organized {
		return types.PhaseRunning
	}
	if prereqs || cached || resolved {
		return types.PhasePending
	}
	return types.PhaseUnknown
}

func mergeMeta(meta *metav1.ObjectMeta, patch *types.PatchMetadata) {
	if patch.Labels != nil {
		if meta.Labels == nil {
			meta.Labels = make(map[string]string)
		}
		for k, v := range patch.Labels {
			meta.Labels[k] = v
		}
	}
	if patch.Annotations != nil {
		if meta.Annotations == nil {
			meta.Annotations = make(map[string]string)
		}
		for k, v := range patch.Annotations {
			meta.Annotations[k] = v
		}
	}
}
