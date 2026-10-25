//go:build js && wasm

// Package main is the WASM entry point for the BlanketOps stub store.
// Build with:
//
//	GOOS=js GOARCH=wasm go build -o blanketops.wasm ./wasm/
//
// The module registers a global `blanketops` object with the following API.
// All functions return a JSON string — parse with JSON.parse() on the JS side.
//
//	blanketops.list(kind, namespace?)            → ListResponse | APIError
//	blanketops.get(kind, namespace, name)        → CRStatus     | APIError
//	blanketops.create(kind, namespace, bodyJSON) → CRStatus     | APIError
//	blanketops.patch(kind, ns, name, bodyJSON)   → CRStatus     | APIError
//	blanketops.delete(kind, namespace, name)     → DeleteResponse | APIError
//	blanketops.events(kind, namespace, name)     → CREvent[]    | APIError
//	blanketops.debug(kind, namespace, name)      → DebugInfo    | APIError
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/blanketops/environments-server/internal/store"
	"github.com/blanketops/environments-server/internal/types"
)

var db = store.NewSeeded()

func main() {
	api := js.ValueOf(map[string]any{})
	js.Global().Set("blanketops", api)

	api.Set("list", js.FuncOf(wasmList))
	api.Set("get", js.FuncOf(wasmGet))
	api.Set("create", js.FuncOf(wasmCreate))
	api.Set("patch", js.FuncOf(wasmPatch))
	api.Set("delete", js.FuncOf(wasmDelete))
	api.Set("events", js.FuncOf(wasmEvents))
	api.Set("debug", js.FuncOf(wasmDebug))

	select {} // keep goroutine alive
}

// ── helpers ───────────────────────────────────────────────────────────────

func toJSON(v any) any {
	b, _ := json.Marshal(v)
	return js.ValueOf(string(b))
}

func apiErr(msg, reason string, code int) any {
	return toJSON(types.APIError{Message: msg, Reason: reason, Code: code})
}

func arg(args []js.Value, i int) string {
	if i < len(args) {
		return args[i].String()
	}
	return ""
}

// ── exported functions ────────────────────────────────────────────────────

// blanketops.list(kind, namespace?) → JSON
func wasmList(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return apiErr("kind required", "BadRequest", 400)
	}
	items, err := db.List(arg(args, 0), arg(args, 1))
	if err != nil {
		return apiErr(err.Error(), "Error", 500)
	}
	if items == nil {
		items = []types.CRStatus{}
	}
	return toJSON(types.ListResponse{Items: items, ResourceVersion: "wasm-rv", Total: len(items)})
}

// blanketops.get(kind, namespace, name) → JSON
func wasmGet(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return apiErr("kind, namespace, name required", "BadRequest", 400)
	}
	cr, err := db.Get(arg(args, 0), arg(args, 1), arg(args, 2))
	if err != nil {
		return apiErr(err.Error(), "NotFound", 404)
	}
	return toJSON(cr)
}

// blanketops.create(kind, namespace, bodyJSON) → JSON
func wasmCreate(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return apiErr("kind, namespace, bodyJSON required", "BadRequest", 400)
	}
	var body types.CreateBody
	if err := json.Unmarshal([]byte(arg(args, 2)), &body); err != nil {
		return apiErr("invalid body: "+err.Error(), "BadRequest", 400)
	}
	cr, err := db.Create(arg(args, 0), arg(args, 1), body)
	if err != nil {
		return apiErr(err.Error(), "Error", 500)
	}
	return toJSON(cr)
}

// blanketops.patch(kind, namespace, name, bodyJSON) → JSON
func wasmPatch(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return apiErr("kind, namespace, name, bodyJSON required", "BadRequest", 400)
	}
	var body types.PatchBody
	if err := json.Unmarshal([]byte(arg(args, 3)), &body); err != nil {
		return apiErr("invalid body: "+err.Error(), "BadRequest", 400)
	}
	cr, err := db.Patch(arg(args, 0), arg(args, 1), arg(args, 2), body)
	if err != nil {
		return apiErr(err.Error(), "Error", 500)
	}
	return toJSON(cr)
}

// blanketops.delete(kind, namespace, name) → JSON
func wasmDelete(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return apiErr("kind, namespace, name required", "BadRequest", 400)
	}
	resp, err := db.Delete(arg(args, 0), arg(args, 1), arg(args, 2))
	if err != nil {
		return apiErr(err.Error(), "NotFound", 404)
	}
	return toJSON(resp)
}

// blanketops.events(kind, namespace, name) → JSON
func wasmEvents(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return apiErr("kind, namespace, name required", "BadRequest", 400)
	}
	cr, err := db.Get(arg(args, 0), arg(args, 1), arg(args, 2))
	if err != nil {
		return apiErr(err.Error(), "NotFound", 404)
	}
	events := cr.Events
	if events == nil {
		events = []types.CREvent{}
	}
	return toJSON(events)
}

// blanketops.debug(kind, namespace, name) → JSON
func wasmDebug(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return apiErr("kind, namespace, name required", "BadRequest", 400)
	}
	cr, err := db.Get(arg(args, 0), arg(args, 1), arg(args, 2))
	if err != nil {
		return apiErr(err.Error(), "NotFound", 404)
	}
	return toJSON(store.BuildDebugInfo(cr))
}
