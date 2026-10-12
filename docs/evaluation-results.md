# Evaluation Results

The authoritative, consolidated evaluation summary. Every number here is
read from an existing evidence file or a test run captured during this
pass (`evidence/final-verification/`) — nothing in this document is
invented. See `docs/evaluation-plan.md` for metric definitions and
methodology, and the evidence directories named in each table for raw
data.

## 1. Functional scenarios

| Scenario | Expected | Observed | Result |
|---|---|---|---|
| Normal ALLOW (`card-allow` / `authorized-passage`) | ALLOW, UNLOCK confirmed, BuildSim reflects UNLOCKED | Exactly that — `evidence/manual-allow-passage/`, `test/e2e/allow_test.go::TestAllow`, and the Docker run in `evidence/final-verification/04-scenario-checks.log` | **PASS** |
| Normal DENY (`card-deny` / `unauthorized-access`) | DENY, door stays LOCKED, no command issued | Exactly that — `evidence/manual-deny-passage/`, `test/e2e/deny_test.go::TestDeny`, and the Docker run in `evidence/final-verification/04-scenario-checks.log` | **PASS** |
| Successful passage (`authorized-passage`, crossing enabled) | ALLOW → UNLOCK → OPEN → occupancy −1/+1 → CLOSE → RELOCK_AFTER_PASSAGE → LOCKED | Full chain observed, both the in-process test and the real-BuildSim/Docker runs | **PASS** |
| Unlock timeout (no open within 10 model s) | `DOOR_NOT_OPENED` event + LOCK (`RELOCK_TIMEOUT`) | Exactly that — `test/e2e/relock_test.go::TestRelockTimeout` | **PASS** |
| After-hours access (`after-hours-access`, 03:00 model time) | ALLOW unaffected; anomaly ANOMALOUS/`UNUSUAL_ACCESS_TIME`; alert true | Exactly that — `evidence/anomaly/`, and reproduced in Docker (`evidence/final-verification/05-decisions-check.log`) | **PASS** |
| Rapid repeat (`rapid-repeat-access`, 3× same card, 1 model-s apart) | First NORMAL, subsequent ANOMALOUS/`REPEATED_ATTEMPT` | Exactly that, confirmed natively (`test/e2e/scenario_test.go`) and in Docker (`evidence/final-verification/05-decisions-check.log`) | **PASS** |
| Burst (`burst-access`, N presentations at a configured rate) | Exactly N observations recorded | `TestScenarioBurstEmitsRequestedCount` asserts N=15; `docs/scenarios.md` documents the mechanism | **PASS** |
| Normal-day (`normal-day`, seeded population sample) | A seeded, reproducible subset/order of the 8-person population presents | `internal/simulator/scenarios_test.go::TestRunScenarioSeedReproducibility`; live Docker run selected `[card-henry, card-frank, card-dave, card-erin, card-carol]` for seed 42, reproducibly | **PASS** |

## 2. Fault experiments

| Fault | Safety behavior | Recovery behavior | Measured recovery | Result |
|---|---|---|---|---|
| Decision Service killed (`SIGKILL`) while an access attempt is in flight | No decision, no command issued; a locked door stays locked (REQ-RES-001) | Fresh process (no carried-over state) resumes producing correct ALLOW/UNLOCK decisions immediately | 10 runs, 2026-10-10 (`evidence/final-verification/10-recovery-time-measurements.log`): recovery time (restart issued → first correct ALLOW/UNLOCK) **min 10.83 ms, mean 18.8 ms, max 32.26 ms**. Real-OS-process capture (`evidence/manual-decision-restart-fault/`) confirms functional correctness; its wall-clock gap is dominated by manual operator pacing, stated honestly as not a clean latency number there. | **PASS** |
| Door Actuator unreachable from BuildSim (real BuildSim killed, and separately the Actuator container stopped) | Command still issued (authorization is logical, not physically gated) but outcome recorded `UNVERIFIED`/empty `observed_state`; never a false `CONFIRMED` (REQ-RES-002) | Restarting the dependency (BuildSim, or the Actuator container) and presenting again produces a full correct ALLOW→UNLOCK→...→LOCKED sequence | Not separately timed (REQ-RES-002 is a correctness property, not a latency one) — `evidence/actuator-failure/`, `evidence/final-verification/09-actuator-failure-and-dashboard.log` | **PASS** |
| Decision→Actuator delivery itself fails (discovered during the stress experiment, concurrency ≥100) | A command is recorded as issued (`issueCommand` posts evidence before attempting delivery) but **no outcome ever follows** — worse than `UNVERIFIED`, which is at least a record that something was attempted | Not implemented/fixed — named as a limitation | N/A | **Known gap, not fixed** (see §4 and `docs/traceability-matrix.md`) |
| Bulk `docker compose restart` (every container, including BuildSim, together) | No unsafe state: BuildSim's door-1 was simply absent (zero persistence by design), not silently mis-commanded | `door-seed`'s one-shot re-seed could race BuildSim's own container restart and fail outright (`curl: (6) Could not resolve host: buildsim`) | Found and fixed within this pass (`compose.yaml` `door-seed` now retries); reproduced twice before the fix, twice confirmed fixed after. `evidence/final-verification/README.md` | **Found, fixed, reverified** |

## 3. Latency (REQ-PERF-001)

Both metrics: `cmd/latencybench -n 200 -interval 20ms`, in-process over loopback with `FakeBuildSimClient` (BuildSim's own responsiveness is not what this metric measures), wall-clock (not model time). `evidence/latency/results.json`.

| Metric | n | p50 (ms) | p95 (ms) | max (ms) | Requirement / threshold |
|---|---|---|---|---|---|
| Access decision latency (observation received → decision received) | 200 | 0.298 | 0.601 | 0.928 | REQ-PERF-001: p95 ≤ 10 ms, p100 ≤ 50 ms — **met with wide margin** (threshold set *after* this measurement, not tuned to pass) |
| End-to-end actuation-confirmation latency (observation received → actuator outcome received) | 200 | 0.884 | 1.791 | 2.21 | Secondary metric, no formal requirement threshold set; reported for completeness. **Caveat**: measured against `FakeBuildSimClient`, not the real BuildSim HTTP adapter — the real adapter's GET-modify-PUT round trip was not separately benchmarked (see §13 of the report / `docs/evaluation-plan.md` §2) |

## 4. Stress / breaking point

`cmd/stressbench -levels=1,10,50,100,200,400,800,1500,3000 -duration=2s`, firing directly at Event/Data Service's `POST /observations` (bypassing the Card Reader's single-poll-loop bottleneck deliberately). `evidence/stress/summary.csv`, full method and root-cause analysis in `evidence/stress/README.md`.

| Concurrency | Offered (req/s) | Throughput (req/s) | p95 (ms) | Error rate | Interpretation |
|---|---|---|---|---|---|
| 1 | 1685 | 1685 | 0.20 | 0.0% | Nominal |
| 10 | 2268 | 2268 | 1.86 | 0.0% | Nominal |
| 50 | 1877 | 1794 | 4.50 | 3.1% | Early degradation begins |
| **100** | 390 | **0** | — | **100%** | **Breaking point**: throughput collapses to zero |
| 200 | 258 | 0 | — | 100% | Total failure persists |
| 400 | 429 | 0 | — | 100% | Total failure persists |
| 800 | 729 | 0 | — | 100% | Total failure persists |
| 1500 | 978 | 0 | — | 100% | Total failure persists |
| 3000 | 4103 | 1687 | 1877.77 | 72.9% | Partial, unstable recovery — connections eventually cycle through, but p95 is 3000× the nominal value |

**Root cause, confirmed not assumed** (full reasoning in `evidence/stress/README.md`): the Decision Service's own log shows `dial tcp ...: can't assign requested address` on its outbound call to the Door Actuator. Every inter-service call goes through `internal/httpx`, which shares `http.DefaultClient` — Go's default transport allows only 2 idle connections per host. Each access attempt fans out into ~4 internal HTTP calls; at concurrency ~100 this is hundreds of simultaneous short-lived loopback connections competing for a two-deep pool per link, churning faster than the OS can recycle them. The same ceiling appeared before *and* after fixing a benchmark-client artifact (undrained response bodies), ruling out the test tool as the cause. **Not optimized in this pass** — the purpose was to find and honestly report the system's limit, not move it; "what would raise the ceiling" is named in `evidence/stress/README.md` but deliberately not implemented.

A second, previously-unknown gap surfaced by this experiment: when Decision→Actuator delivery itself fails (not Actuator→BuildSim, which REQ-RES-002 already covers), a command is recorded as issued with **no outcome ever following** — see §2 above.

## 5. Anomaly behavior (REQ-DEC-002/003/005/006)

Deterministic rule-based baseline (`internal/decision/anomaly.go`): two weighted factors (unusual access time outside 07:00–19:00 model time, repeated attempts within 5 model seconds), summed, classified `ANOMALOUS` at score ≥ 0.5. `anomaly_result` never affects `final_action` — the load-bearing invariant (REQ-DEC-003), structurally true (`authorize()` and the anomaly scorer never read each other's output), not merely asserted.

| Scenario | Card / model time | Authorization | Anomaly classification | Final policy action | Alert |
|---|---|---|---|---|---|
| Normal authorized | card-allow, 10:00 (day) | ALLOW / `ROLE_ALLOW` | NORMAL | **ALLOW** | false |
| Anomalous but authorized | card-allow, 03:00 (night) | ALLOW / `ROLE_ALLOW` | ANOMALOUS / `UNUSUAL_ACCESS_TIME` | **ALLOW** | true |
| Unauthorized (also anomalous hour) | card-deny, 03:00 (night) | DENY / `NO_PERMISSION` | ANOMALOUS / `UNUSUAL_ACCESS_TIME` | **DENY** | true |
| Rapid repeat, 1st attempt | card-alice, 10:00 | ALLOW / `ROLE_ALLOW` | NORMAL (no prior attempt yet) | **ALLOW** | false |
| Rapid repeat, 2nd attempt (1 model-s later) | card-alice, 10:00:01 | ALLOW / `ROLE_ALLOW` | ANOMALOUS / `REPEATED_ATTEMPT` | **ALLOW** | true |
| Rapid repeat, 3rd attempt (2 model-s later) | card-alice, 10:00:02 | ALLOW / `ROLE_ALLOW` | ANOMALOUS / `REPEATED_ATTEMPT` | **ALLOW** | true |

Row 3 (unauthorized + anomalous hour) is the direct proof of REQ-DEC-003:
the anomaly classification is reported honestly either way, but a DENY
stays a DENY regardless. `evidence/anomaly/`, `internal/decision/anomaly_test.go`,
`internal/decision/server_decide_test.go`, and the Docker reproduction in
`evidence/final-verification/05-decisions-check.log` all agree.

**Limitation, stated plainly** (unchanged from the original evaluation
plan): a third candidate factor (unusual destination/zone) is not
implemented — one controlled door means nothing exists for it to vary
against, so adding it now would be untestable, not genuinely evaluated.
Thresholds (0.6 per factor, 0.5 cutoff, the time window, the repeat
window) are a defensible starting baseline, not derived from a real
evaluation data set — none exists for this project.

## Known, preserved findings (not fixed, by design of this pass)

- **HTTP connection-pool stress bottleneck** (§4): confirmed root cause,
  not optimized. See `evidence/stress/README.md`.
- **Decision→Actuator missing-outcome failure mode** (§2, §4): a command
  can be recorded with no resolution if delivery to the Actuator itself
  fails. Not fixed; named as a limitation.
- **Door-seed vs. bulk restart race** (§2): found *and* fixed this pass
  (the one exception — see `evidence/final-verification/README.md` for
  why this one was judged safe and small enough to correct immediately
  rather than only document).
