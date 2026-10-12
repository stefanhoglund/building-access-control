# Performance experiment: access decision latency (REQ-PERF-001)

## Metric definitions

**Access decision latency** = wall-clock time from the Event/Data Service
receiving an access-attempt observation to the Event/Data Service
receiving the resulting AccessDecision (`decision.recorded_at -
observation.recorded_at`, matched by `event_id`). Wall-clock, not model
time.

**End-to-end actuation-confirmation latency** (secondary metric,
distinguished from the above) = wall-clock time from the same observation
to the Door Actuator's outcome being received by Event/Data
(`outcome.recorded_at - observation.recorded_at`, matched through the
decision's `decision_id` → the UNLOCK command's `command_id` → its
outcome).

Both are computed directly from existing evidence-record timestamps; no
production code was instrumented to produce them.

## Reproduction

```bash
go run ./cmd/latencybench -n 200 -interval 20ms
```

Uses `FakeBuildSimClient` and five of the real components in-process
(crossing disabled, so each attempt is exactly one `ALLOW`→`UNLOCK`
cycle). Writes `evidence.jsonl`, `results.json` (full stats + raw
samples), and `samples.csv` to this directory, overwriting the previous
run.

## Baseline result (200 samples, 2026-10-09)

| Metric | n | min | p50 | p95 | max |
|---|---|---|---|---|---|
| Decision latency (ms) | 200 | 0.16 | 0.30 | 0.60 | 0.93 |
| Actuation confirmation latency (ms) | 200 | 0.53 | 0.88 | 1.79 | 2.21 |

200/200 attempts matched a complete decision; 200/200 matched a complete
actuation outcome (no drops at this load).

## Threshold

`docs/requirements.md` REQ-PERF-001 sets the acceptance threshold at
**p95 ≤ 10 ms** (and p100 ≤ 50 ms), chosen *after* this measurement, not
before it — see that requirement's text for the full justification.

## Reading these numbers honestly

This baseline uses `FakeBuildSimClient` and runs everything on one
machine over loopback — which is also how the real system is deployed in
this course (every process runs on the same laptop), so the *decision*
latency number (which never touches BuildSim) is representative of real
operation, not just a synthetic lab figure. The *actuation confirmation*
number would be somewhat higher against the real BuildSim adapter (an
extra GET-modify-PUT HTTP round trip per command instead of an in-memory
map write) — this has not yet been separately measured against the real
BuildSim instance and is named as a limitation rather than assumed
negligible.
