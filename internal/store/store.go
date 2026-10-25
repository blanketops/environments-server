package store

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/blanketops/environments-server/internal/types"
)

// KnownKinds is the complete list of supported CR kinds.
var KnownKinds = []string{
	"serviceunits", "deployments", "gitrepositories",
	"githubevents", "routes", "domains",
	"supplychains", "githubwebhooks",
}

// Store is a thread-safe in-memory CRUD store for all CR kinds.
type Store struct {
	mu   sync.RWMutex
	data map[string][]types.CRStatus
}

func New() *Store {
	s := &Store{data: make(map[string][]types.CRStatus)}
	for _, k := range KnownKinds {
		s.data[k] = []types.CRStatus{}
	}
	return s
}

func NewSeeded() *Store {
	s := New()
	Seed(s)
	return s
}

func (s *Store) isKnown(kind string) bool {
	_, ok := s.data[kind]
	return ok
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }

// ── CRUD ──────────────────────────────────────────────────────────────────

func (s *Store) List(kind, namespace string) ([]types.CRStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.isKnown(kind) {
		return nil, storeErr(400, fmt.Sprintf("unknown kind: %s", kind), "BadRequest")
	}
	items := s.data[kind]
	if namespace == "" {
		out := make([]types.CRStatus, len(items))
		copy(out, items)
		return out, nil
	}
	var out []types.CRStatus
	for _, item := range items {
		if item.Namespace == namespace {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *Store) Get(kind, namespace, name string) (types.CRStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.isKnown(kind) {
		return types.CRStatus{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
	}
	for _, item := range s.data[kind] {
		if item.Name == name && (namespace == "" || item.Namespace == namespace) {
			return item, nil
		}
	}
	return types.CRStatus{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
}

func (s *Store) Create(kind, defaultNS string, body types.CreateBody) (types.CRStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isKnown(kind) {
		return types.CRStatus{}, storeErr(400, fmt.Sprintf("unknown kind: %s", kind), "BadRequest")
	}
	name := body.Metadata.Name
	if name == "" {
		name = fmt.Sprintf("%s-%d", kind, time.Now().UnixMilli())
	}
	ns := body.Metadata.Namespace
	if ns == "" {
		ns = defaultNS
	}
	for _, item := range s.data[kind] {
		if item.Name == name {
			return types.CRStatus{}, storeErr(409, fmt.Sprintf("%s/%s already exists", kind, name), "AlreadyExists")
		}
	}
	t := nowISO()
	cr := types.CRStatus{
		Name: name, Namespace: ns, Kind: kind,
		Phase: types.PhasePending, Generation: 1,
		Labels: body.Metadata.Labels,
		Spec:   body.Spec,
		Conditions: []types.Condition{{
			Type: "Ready", Status: "False", Reason: "Initializing",
			Message:            "Resource created — reconciliation starting",
			LastTransitionTime: t,
		}},
		Events: []types.CREvent{{
			Type: "Normal", Reason: "Created",
			Message:   fmt.Sprintf("%s created via BlanketOps API", kind),
			Count:     1,
			FirstTime: t, LastTime: t,
		}},
	}
	s.data[kind] = append([]types.CRStatus{cr}, s.data[kind]...)
	return cr, nil
}

func (s *Store) Patch(kind, namespace, name string, body types.PatchBody) (types.CRStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isKnown(kind) {
		return types.CRStatus{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
	}
	for i, item := range s.data[kind] {
		if item.Name == name && (namespace == "" || item.Namespace == namespace) {
			updated := item
			updated.Generation++
			if body.Spec != nil {
				updated.Spec = body.Spec
			}
			if body.Metadata != nil {
				if updated.Labels == nil {
					updated.Labels = make(map[string]string)
				}
				for k, v := range body.Metadata.Labels {
					updated.Labels[k] = v
				}
				if updated.Annotations == nil {
					updated.Annotations = make(map[string]string)
				}
				for k, v := range body.Metadata.Annotations {
					updated.Annotations[k] = v
				}
			}
			updated.Conditions = []types.Condition{{
				Type: "Ready", Status: "False", Reason: "Patching",
				Message:            "Spec updated — reconciliation starting",
				LastTransitionTime: nowISO(),
			}}
			s.data[kind][i] = updated
			return updated, nil
		}
	}
	return types.CRStatus{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
}

func (s *Store) Delete(kind, namespace, name string) (types.DeleteResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isKnown(kind) {
		return types.DeleteResponse{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
	}
	items := s.data[kind]
	for i, item := range items {
		if item.Name == name && (namespace == "" || item.Namespace == namespace) {
			s.data[kind] = append(items[:i], items[i+1:]...)
			return types.DeleteResponse{Name: item.Name, Namespace: item.Namespace, Kind: kind, Deleted: true}, nil
		}
	}
	return types.DeleteResponse{}, storeErr(404, fmt.Sprintf("%s/%s not found", kind, name), "NotFound")
}

// SeedKind replaces the item list for a given kind (used by Seed and tests).
func (s *Store) SeedKind(kind string, items []types.CRStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[kind] = items
}

// ── Debug ─────────────────────────────────────────────────────────────────

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

// ── storeError ─────────────────────────────────────────────────────────────

type storeError struct {
	status  int
	message string
	reason  string
}

func storeErr(status int, message, reason string) *storeError {
	return &storeError{status: status, message: message, reason: reason}
}

func (e *storeError) Error() string      { return e.message }
func (e *storeError) HTTPStatus() int    { return e.status }
func (e *storeError) HTTPReason() string { return e.reason }
