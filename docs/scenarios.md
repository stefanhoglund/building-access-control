# Scenarios

Source of truth for simulation behavior and the named, deterministic
evaluation scenarios (CLAUDE.md §1 item 5, design-specification.md §3.5
Scenario). This document describes what each scenario does and why;
`config/scenarios.json` is the executable definition, and
`internal/simulator/scenarios.go` is the (intentionally small) code that
executes it.

## 1. Simulated population

`config/population.json` configures a small, deterministic, behaviorally
diverse set of simulated people (design-specification.md §3.1
SimulatedPerson / §3.2 Schedule). The point is behavioral diversity, not
population size: eight people, spanning every role this project's
authorization configuration defines (`config/system-config.json`), each
with its own "normal" access-hour window used only for scenario
generation — never for authorization, which is decided purely from the
card/user/role/permission data in `system-config.json`.

| person_id | card_id      | role (display only) | normal hours | authorized on door-1? |
|-----------|--------------|----------------------|--------------|------------------------|
| alice     | card-alice   | employee             | 08–17        | yes (ROLE_ALLOW)       |
| bob       | card-bob     | employee             | 07–16        | yes (ROLE_ALLOW)       |
| carol     | card-carol   | facilities           | 06–14        | yes (ROLE_ALLOW)       |
| dave      | card-dave    | manager              | 08–18        | yes (ROLE_ALLOW)       |
| erin      | card-erin    | visitor              | 10–16        | no (NO_PERMISSION)     |
| frank     | card-frank   | visitor              | 10–16        | no (NO_PERMISSION)     |
| grace     | card-grace   | student              | 09–20        | no (NO_PERMISSION)     |
| henry     | card-henry   | employee             | 08–17        | yes (ROLE_ALLOW)       |

`card-allow`/`card-deny` (and their users `user-allow`/`user-deny`) remain
in `system-config.json` unchanged and are not part of the population —
they are the original, pre-existing fixtures the unit/e2e test suite
(`test/e2e/allow_test.go`, `deny_test.go`, etc.) depends on by name.

## 2. Invoking a scenario

`POST /scenario/run` on the Simulator (`docs/data-contracts.md` §10):

```bash
curl -X POST http://localhost:8081/scenario/run \
  -H 'Content-Type: application/json' \
  -d '{"scenario":"normal-day","seed":42}'
```

or via `scripts/run-local.sh demo-normal-day 42` (and the equivalent
`demo-*` subcommands for the other named scenarios — see
`scripts/run-local.sh`, no args needed, or `make docker-demo-*` once
Docker Compose is running). The response and the
`evidence/scenarios/runs.jsonl` log both record `run_id`, `scenario`,
`seed`, `model_start`, and the exact `card_ids` (in order) the run drove —
enough to identify and reproduce any run without adding scenario/seed
fields to the core evidence contracts (§10.1 of `data-contracts.md`).

## 3. Named scenarios

All eight required scenarios are defined in `config/scenarios.json`.
Adding or adjusting one is a configuration change, not a code change,
except for a genuinely new *kind* of execution strategy (see §4).

### normal-day
Several authorized users generate normal accesses spread across a model
day (REQ-SIM-001/002). `kind: "population_sample"`: a seeded random
subset of the configured population (`sample_size: 5` of 8), in a seeded
random order, spaced `spacing_seconds` apart with seeded jitter. The
seed determines both which people are picked and the order/timing —
verified reproducible by `internal/simulator/scenarios_test.go`'s
`TestRunScenarioSeedReproducibility`.

### authorized-passage
One authorized user (alice), full successful-passage sequence:
ALLOW → UNLOCK → OPEN → occupant crosses → occupancy updates → CLOSE →
RELOCK_AFTER_PASSAGE (REQ-FUNC-004, REQ-SIM-005/006). `kind: "fixed"`,
one step.

### unauthorized-access
A user/card without permission on this door attempts entry (erin,
visitor role, `NO_PERMISSION`). Authorization DENY, no unlock, no
crossing (REQ-SIM-004).

### after-hours-access
A normally-authorized user (alice) accesses at an unusual model time
(03:00, outside the anomaly module's 07:00–19:00 normal window,
`internal/decision/anomaly.go`). Authorization stays ALLOW (anomaly
detection never affects permission, REQ-DEC-003); anomaly classification
is ANOMALOUS (`UNUSUAL_ACCESS_TIME`), `alert: true`.

### rapid-repeat-access
The same person (alice) presents three times, one model-second apart —
well inside the anomaly module's 5-second repeated-attempt window. The
first presentation has no prior attempt on record and is NORMAL; the
second and third are each classified ANOMALOUS (`REPEATED_ATTEMPT`).

### burst-access
One person (alice) presenting a configurable number of times at a
configurable rate, for latency/stress evaluation
(`cmd/latencybench`/`cmd/stressbench` cover the same ground with
finer-grained control; this scenario exists so the same effect is
reachable through the ordinary scenario interface). `count`/`interval_ms`
in the `/scenario/run` request override the scenario file's
`default_count`/`default_interval_ms`.

### decision-service-failure
A single authorized presentation (alice), intended to be issued while the
Decision Service is killed/restarted around the call — the scenario
definition itself does not perform process control (the Simulator must
not reach outside its own domain to manage other processes); orchestrating
the kill/restart is `scripts/run-local.sh demo-decision-failure`'s job.
This is the same fault experiment documented in
`evidence/decision-restart-fault/` and
`evidence/manual-decision-restart-fault/`, now reproducible through a
named scenario rather than only through a raw `/scenario/present` call.

### actuator-or-buildsim-failure
A single authorized presentation (alice), intended to be issued while the
Door Actuator or BuildSim is stopped around the call —
`scripts/run-local.sh demo-actuator-failure` orchestrates the
stop/restart. Same experiment as `evidence/actuator-failure/`.

## 4. Scope and limitations

This is deliberately not a general scenario engine:

- no path planning, no crowd physics, no multi-door routing (this
  project simulates exactly one controlled door, REQ-TOPO-004);
- no full `BehaviorProfile` (design-specification.md §3.3) — no
  probabilistic destinations/dwell-times/transition probabilities, only
  the three execution kinds above (`fixed`, `population_sample`,
  `burst`);
- `Schedule` (§3.2) is reduced to one normal-hour window per person, used
  only by this document/`config/population.json` as descriptive
  metadata — it is not read by any code path; the anomaly module's own
  fixed 07:00–19:00 window (`internal/decision/anomaly.go`) is what
  actually drives the after-hours scenario's classification.

REQ-SIM-001/002 are therefore **partially** verified by this
implementation (reproducible, seed-driven selection/ordering of a
configured population — not full configurable arrival/departure/lunch
distributions); REQ-SIM-003 (configurable, reproducible seed) is fully
verified. See `docs/test-plan.md`.
