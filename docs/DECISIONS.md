# Architecture decisions

These are decisions illustrated by this standalone synthetic lab, not claims about ownership, deployment or measured outcomes of a commercial system.

| Problem | Choice | Alternative | Trade-off |
| --- | --- | --- | --- |
| Heterogeneous partner formats | Canonical order + pure mapping boundary | Partner-specific intake model | A central contract requires versioning and partner tests. |
| Repeated intake requests | Idempotency key + payload conflict detection | Treat each request as new | Real distributed idempotency requires durable storage and transactional coordination. |
| Remote work can fail independently | An in-memory outbox and deterministic processing loop | Direct synchronous delivery | This illustrates acceptance separately from delivery; real systems require a durable transactional outbox and message deduplication. |
| Supplier business decision differs from delivery | Independent technical and commercial states | A single order status | More states need explicit invariants and tests. |
| Transient failure | Bounded retry and observable terminal failure | Retry forever or silently discard | A production design needs backoff, reconciliation and carefully chosen retry policies. |
| Operator insight | Explicit timeline events | Only final status | Real audit logs require appropriate durability, access control and retention. |

## Verification

`go test ./...` exercises duplicate intake, conflicting idempotency keys, mapping, bounded retry, rejected early confirmation, terminal failure, repeated confirmation and immutable record snapshots. `go run ./cmd/demo` shows a deterministic timeline with one simulated transient failure and one business decision.

## Scope

The demonstration intentionally does not include networking, real supplier integrations, secrets, tenancy, deployment, authentication or production persistence. The example runs sequentially in one process. Expanding it into a distributed system would require design and independent verification around crash consistency, concurrency, idempotent external effects, tenant separation, observability, access controls and capacity.
