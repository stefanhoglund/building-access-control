# Breaking-point experiment: concurrent access attempts against the single Decision Service pipeline

## Method

`cmd/stressbench` fires access-attempt observations directly at the
Event/Data Service's `POST /observations` (bypassing the Card Reader and
the Simulator's single-reader poll queue on purpose — that queue is a
known, separate, uninteresting bottleneck that would dominate any
measurement taken through it) from N concurrent goroutines, sustained for
a fixed duration per level, sweeping N = 1, 10, 50, 100, 200, 400, 800,
1500, 3000. At each level it measures: offered load (requests/s sent),
throughput (requests/s that produced a matched `AccessDecision`), p50/p95
decision latency, and the send-error rate.

```bash
go run ./cmd/stressbench -levels=1,10,50,100,200,400,800,1500,3000 -duration=2s
```

## Result

| Concurrency | Offered (req/s) | Throughput (req/s) | p95 (ms) | Error rate |
|---|---|---|---|---|
| 1 | 1685 | 1685 | 0.20 | 0% |
| 10 | 2268 | 2268 | 1.86 | 0% |
| 50 | 1877 | 1794 | 4.50 | 3.1% |
| **100** | 390 | **0** | — | **100%** |
| 200 | 258 | 0 | — | 100% |
| 400 | 429 | 0 | — | 100% |
| 800 | 729 | 0 | — | 100% |
| 1500 | 978 | 0 | — | 100% |
| 3000 | 4103 | 1687 | 1877.77 | 72.9% |

Full per-level evidence and raw samples: `summary.json`, `summary.csv`,
`level-<N>/evidence.jsonl`.

## Where it breaks, and why

Between concurrency 50 and 100, throughput collapses from 1794 req/s to
**zero**, with the Decision Service logging:

```
decision: post command to actuator: httpx: POST http://127.0.0.1:.../commands: dial tcp ...: can't assign requested address
```

This is ephemeral-port/connection exhaustion on the **Decision Service's
own outbound call to the Door Actuator** — not BuildSim, not the evidence
file, not the authorization or anomaly logic. The root cause: every
service-to-service call in this system goes through `internal/httpx`,
which uses `http.DefaultClient` — Go's default `http.Transport` allows
only **2 idle connections per host**. Each incoming observation fans out
into four internal HTTP calls (Event/Data→Decision, Decision→Event/Data
×2 for decision/command evidence, Decision→Actuator, Actuator→Event/Data
for the outcome); at ~100 concurrent in-flight attempts that is several
hundred simultaneous short-lived connections competing for a two-deep
pool per link, and the connections churn faster than the OS can recycle
them on loopback.

**This was confirmed, not assumed.** The same ceiling appeared on the
first sweep using a naive benchmark client that didn't drain response
bodies before closing them (itself a classic Go cause of connection-reuse
failure, which would have been a measurement artifact, not a system
property). The benchmark client was fixed — bodies are now drained, and
its own transport was given a 2000-connection pool specifically so it
could never be the bottleneck — and the sweep was rerun. The ceiling
did not move. The error message's origin (the Decision Service's own log
line, not the benchmark tool) and its unchanged position after fixing the
one plausible tooling confound both point to the production `internal/httpx`
client as the real limit, not an artifact of the experiment.

At concurrency 3000, throughput partially recovers (1687 req/s) as enough
connections eventually cycle through, but the error rate stays high
(73%) — consistent with intermittent, not total, exhaustion at extreme
overload rather than a second, different failure mode.

## A gap this experiment surfaced (not previously known)

When `Decision → Actuator` delivery itself fails (as opposed to the
Actuator reaching BuildSim and failing there, which `REQ-RES-002` already
covers and `evidence/actuator-failure/` already demonstrates), the
`ActuatorCommand` is still recorded as evidence — `issueCommand` posts it
to the Event/Data Service *before* attempting delivery to the Actuator —
but **no `ActuatorOutcome` ever follows**, because the Actuator never
received the command to report one. This is a command recorded as issued
with no resolution at all, which is a different and arguably worse gap
than "unverified": at least an `UNVERIFIED` outcome is a record that
something was attempted. This was not exercised or known before this
stress experiment; it is named here as a limitation, not fixed, since
fixing it is new work beyond this experiment's scope.

## What would raise the ceiling (not implemented — named, not built)

Giving `internal/httpx`'s shared client a larger per-host connection pool
(the same fix already applied to the benchmark tool) is the obvious,
minimal first step and would very likely move this specific ceiling
substantially higher, since the bottleneck is a configuration default,
not a structural property of the architecture. It was not done here:
this experiment's purpose is to find and honestly report the limit of
the system as built, not to optimize past it.
