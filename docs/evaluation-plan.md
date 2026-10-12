# Evaluation Plan

This document defines what is measured, how, and with which clock, and
points to the raw evidence for each experiment. It is the plan; the
numbers themselves live under `evidence/` (one directory per experiment,
each with its own `README.md` giving the exact reproduction commands).

## 0. Named scenarios driving this plan

`config/scenarios.json` (`docs/scenarios.md`) defines eight named, seeded,
reproducible scenarios, invoked via `POST /scenario/run` on the Simulator.
Each section below is driven by one or more of them, in addition to the
unit/e2e `go test` suite already cited per section:

| Scenario | Drives |
|---|---|
| `authorized-passage` | §1 nominal correctness (ALLOW + full passage) |
| `unauthorized-access` | §1 nominal correctness (DENY, no crossing) |
| `normal-day` | §1 nominal correctness under a varied, seeded population (REQ-SIM-001/002/003) |
| `after-hours-access` | §5 anomaly evaluation (unusual-time factor) |
| `rapid-repeat-access` | §5 anomaly evaluation (repeated-attempt factor) |
| `burst-access` | §6 stress/breaking-point, as an alternative driver reachable through the ordinary scenario interface rather than only `cmd/stressbench` |
| `decision-service-failure` | §3 recovery time — scenario definition/reproduction support; the actual kill/restart is orchestrated by `scripts/run-local.sh demo-decision-failure` / `make docker-demo-decision-failure`, not by the scenario itself |
| `actuator-or-buildsim-failure` | §4 actuator/BuildSim failure — same pattern, via `scripts/run-local.sh demo-actuator-failure` |

Scenario reproducibility (run_id, scenario, seed, model_start,
card_ids) is recorded in `evidence/scenarios/runs.jsonl`
(`docs/data-contracts.md` §10.1), separately from the Historical Evidence
Store. All eight scenarios were also exercised against the Docker Compose
deployment (`docs/deployment.md`), confirming the same evaluation
evidence is reachable identically in both deployment modes.

## 1. Nominal correctness

**What.** The system produces a correct ALLOW or DENY for an access
attempt, and a correct successful-passage sequence (unlock → open →
cross → close → relock) when authorized.

**How.** `test/e2e/allow_test.go`, `deny_test.go`, `relock_test.go`,
`passage_test.go`; `test/integration/realbuildsim_test.go` (same ALLOW/
relock sequence against the real BuildSim binary, not the fake); manual
captures `evidence/manual-allow-passage/`, `evidence/manual-deny-passage/`.

**Clock.** Not applicable (correctness, not timing).

## 2. Decision latency (REQ-PERF-001)

**Metric definition** (exact, as required before any threshold is
chosen):

> **Access decision latency** = wall-clock time from the Event/Data
> Service receiving an access-attempt observation to the Event/Data
> Service receiving the resulting AccessDecision
> (`decision.recorded_at − observation.recorded_at`, matched by
> `event_id`).

**Clock.** Wall-clock, explicitly not model time (design-specification.md
§4.3: these must never be conflated).

**Secondary metric, clearly distinguished:**

> **End-to-end actuation-confirmation latency** = wall-clock time from
> the same observation to the Door Actuator's outcome being received by
> Event/Data (`outcome.recorded_at − observation.recorded_at`).

**How.** `go run ./cmd/latencybench -n 200 -interval 20ms` — drives 200
real presentations through five real components (`FakeBuildSimClient`;
BuildSim's own responsiveness is not what this metric is about) and
computes both distributions directly from evidence-record timestamps, no
production code instrumented.

**Threshold.** `docs/requirements.md` REQ-PERF-001: p95 ≤ 10 ms, chosen
*after* observing the baseline (p95 = 0.60 ms), not before — see that
requirement's text for the full justification, and
`evidence/latency/README.md` for the methodology and the honest caveat
that the actuation-confirmation number has not been separately measured
against the real BuildSim adapter.

## 3. Recovery time (REQ-RES-004)

**What.** How long after the Autonomous Decision Service is restarted
does the system resume producing correct decisions, and does a locked
door stay locked for the whole outage (REQ-RES-001).

**Metric.** Wall-clock time from the restarted process beginning to
listen to the first subsequent access attempt producing a correct,
confirmed ALLOW/UNLOCK.

**How.** `test/e2e/decision_restart_test.go` (10 real runs, in-process:
the Decision Service's listener and ticker are stopped and a *fresh*
instance bound to the same address, no carried-over state) and a real
OS-process capture (`SIGKILL` against the actual compiled binary, real
BuildSim) in `evidence/manual-decision-restart-fault/`.

**Result.** 10.83–32.26 ms, mean 18.8 ms (automated, confound-free; 10
fresh runs, `evidence/final-verification/10-recovery-time-measurements.log`).
The real-process capture
confirms correctness but its wall-clock gap is dominated by manual
operator pacing between commands, not system latency — stated explicitly
in that directory's `README.md` rather than reported as if it were a
clean measurement.

## 4. Actuator / BuildSim failure (REQ-RES-002)

**What.** When a command cannot be physically confirmed (BuildSim
unreachable), the system must record `UNVERIFIED` and must never claim a
false `CONFIRMED`.

**How.** `internal/actuator/server_test.go::TestHandleCommand_BuildSimUnavailable_RecordsUnverifiedNotFalseSuccess`
(deterministic, a `BuildSimClient` test double that always fails) and a
real capture — the actual BuildSim process killed mid-ALLOW, then
restarted and re-seeded — in `evidence/actuator-failure/`.

**Clock.** Not applicable (a correctness/safety property, not a timing
measurement).

**System-level observation.** BuildSim is a single shared dependency for
three different components (Actuator, Simulator, DoorStateSensor); see
`evidence/actuator-failure/README.md` for why this is named as a
limitation rather than silently worked around.

## 5. Anomaly evaluation (REQ-DEC-002/003/005/006)

**What.** The deterministic anomaly baseline (`internal/decision/anomaly.go`)
correctly classifies behavior as `NORMAL`/`ANOMALOUS` using two factors
(unusual access time, repeated attempts), and — the architectural
invariant under test — never lets that classification change
`final_action`.

**How.** `internal/decision/anomaly_test.go` (scorer in isolation, 6
cases) and `server_decide_test.go` (3 required end-to-end scenarios:
normal authorized, anomalous-but-authorized, unauthorized — the last
deliberately also anomalous, to prove DENY is never overridden). Live
captures of all three scenarios against the real running system, dashboard
included, in `evidence/anomaly/`.

**Clock.** Model time (the access-time window and repeated-attempt
window are both evaluated against model time, per the project's existing
time-semantics convention — REQ-AUTH-005-style permission windows already
use model time, and this follows the same rule).

**Limitation, stated plainly.** A third candidate factor (unusual
destination/zone) is not implemented: one controlled door means nothing
exists for it to vary against, so adding it now would be untestable.
Thresholds are a defensible starting baseline, not derived from a real
evaluation data set.

## 6. Stress / breaking-point experiment (grade-5 evidence)

**What.** Concurrent access attempts against the single Decision Service
pipeline, swept from 1 to 3000 concurrent senders, measuring offered
load, throughput, p95 decision latency, and error rate at each level,
until a clear breaking point appears.

**How.** `go run ./cmd/stressbench -levels=1,10,50,100,200,400,800,1500,3000 -duration=2s`
— fires observations directly at Event/Data Service from N concurrent
goroutines (bypassing the Card Reader's single-poll-loop bottleneck
deliberately, since that is a separate, uninteresting limit that would
mask the pipeline's own concurrency behavior).

**Result.** Throughput collapses from 1794 req/s (concurrency 50) to
**zero** (concurrency 100–1500), recovering partially but with a 73%
error rate at concurrency 3000. Root cause, confirmed (not assumed) by
first ruling out a benchmark-client artifact: `internal/httpx`'s shared
use of `http.DefaultClient` (2 idle connections per host) exhausts
ephemeral loopback ports under the ~4x internal HTTP fan-out per access
attempt. Full methodology, the artifact that was found and fixed before
trusting the result, and a previously-unknown gap this experiment
surfaced (a command can be recorded as issued with no outcome ever
following, if Decision→Actuator delivery itself fails) are in
`evidence/stress/README.md`.

**Clock.** Wall-clock (request send time to matched-decision time).

## Evidence directory index

| Directory | Experiment |
|---|---|
| `evidence/manual-allow-passage/` | Nominal ALLOW + successful passage, real system |
| `evidence/manual-deny-passage/` | Nominal DENY, real system |
| `evidence/decision-restart-fault/` | REQ-RES-001/004, automated (10 runs) |
| `evidence/manual-decision-restart-fault/` | REQ-RES-001/004, real OS process |
| `evidence/actuator-failure/` | REQ-RES-002, automated + real BuildSim kill |
| `evidence/latency/` | REQ-PERF-001 |
| `evidence/anomaly/` | REQ-DEC-002/003/005/006, three required scenarios |
| `evidence/stress/` | Breaking-point / concurrency experiment |
| `evidence/scenarios/runs.jsonl` | Reproducibility log for every named `/scenario/run` invocation (not the Historical Evidence Store — §10.1 of `docs/data-contracts.md`) |
