package types

import "encoding/json"

// Phase constants mirror the frontend Phase type.
type Phase = string

const (
	PhaseReady   Phase = "Ready"
	PhaseRunning Phase = "Running"
	PhasePending Phase = "Pending"
	PhaseFailed  Phase = "Failed"
	PhaseUnknown Phase = "Unknown"
)

// ── Core CR types ──────────────────────────────────────────────────────────

type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"` // "True" | "False" | "Unknown"
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"lastTransitionTime"`
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
}

type CREvent struct {
	Type      string `json:"type"` // "Normal" | "Warning"
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Count     int    `json:"count"`
	FirstTime string `json:"firstTime"`
	LastTime  string `json:"lastTime"`
}

type DebugInfo struct {
	Name                    string          `json:"name"`
	Namespace               string          `json:"namespace"`
	Kind                    string          `json:"kind"`
	QueueDepth              int             `json:"queueDepth"`
	ReconcileAttempts       int             `json:"reconcileAttempts"`
	LastReconcileDurationMs int             `json:"lastReconcileDurationMs"`
	LastReconcileTime       string          `json:"lastReconcileTime"`
	DispatchRoute           string          `json:"dispatchRoute"`
	MediatorChain           []string        `json:"mediatorChain"`
	WorkerSlot              int             `json:"workerSlot"`
	LastError               *string         `json:"lastError,omitempty"`
	Predicates              map[string]bool `json:"predicates"`
}

type CRStatus struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Kind        string            `json:"kind"`
	Phase       string            `json:"phase"`
	Generation  int64             `json:"generation"`
	Conditions  []Condition       `json:"conditions"`
	Events      []CREvent         `json:"events"`
	Spec        json.RawMessage   `json:"spec,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ── API response types ────────────────────────────────────────────────────

type ListResponse struct {
	Items           []CRStatus `json:"items"`
	ResourceVersion string     `json:"resourceVersion"`
	Continue        string     `json:"continue,omitempty"`
	Total           int        `json:"total"`
}

type DeleteResponse struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Deleted   bool   `json:"deleted"`
}

type APIError struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Code    int    `json:"code"`
}

// ── Request body types ────────────────────────────────────────────────────

type CreateBody struct {
	Metadata CreateMetadata  `json:"metadata"`
	Spec     json.RawMessage `json:"spec,omitempty"`
}

type CreateMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type PatchBody struct {
	Metadata *PatchMetadata  `json:"metadata,omitempty"`
	Spec     json.RawMessage `json:"spec,omitempty"`
}

type PatchMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}
