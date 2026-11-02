## What

<!-- One or two sentences. What does this PR change? -->

## Why

<!-- The intent. Link the issue if one exists: Closes #123 -->

## Domain

<!-- Mark all that apply -->

* [ ] `server` — REST API server
* [ ] `api` — Kubernetes API types
* [ ] `ui` — Embedded React UI
* [ ] CI / tooling / docs

## API impact

* [ ] No API surface change
* [ ] `v1alpha1` — free to change
* [ ] `v1beta1` — backwards-compatible only, deprecations allowed
* [ ] `v1` — **breaking change** (requires version bump + changelog entry + migration note)

<!-- If breaking: what breaks, and what must consumers do? -->

## Checklist

* [ ] `go build` and `go vet` pass locally
* [ ] `go test ./...` passes
* [ ] `golangci-lint` passes
* [ ] Business logic lives in appropriate domain module
* [ ] Import paths use `github.com/blanketops/environments-api/api/...` for CRD types
* [ ] BlanketOps labels present where required (`environments.blanketops.dev/*`)
* [ ] Commit messages follow Conventional Commits

## Notes for reviewer

<!-- Anything non-obvious: design trade-offs, deferred follow-ups, areas needing close attention -->