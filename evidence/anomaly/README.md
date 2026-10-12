# Anomaly evaluation: three required scenarios

Captured against the live, fully-restarted seven-process system plus the
dashboard and real BuildSim, not a synthetic test. See
`internal/decision/anomaly_test.go` and
`internal/decision/server_decide_test.go` for the corresponding automated
tests (including the reverse case — a repeated attempt rather than an
unusual hour — and the specific invariant check that anomaly can never
turn a DENY into an ALLOW).

## Method

Deterministic rule-based baseline (`internal/decision/anomaly.go`,
design-specification.md §6/§12.4): two weighted factors — unusual access
time (outside 07:00–19:00 model time) and repeated attempts (same card
within 5 model seconds of its last attempt) — summed and classified
`ANOMALOUS` at a total score ≥ 0.5. Not a learned model; no ML dependency.
`anomaly_result`/`anomaly_reason`/`anomaly_score` are recorded in every
`AccessDecision`, regardless of classification.

## The three scenarios (model time advanced deterministically, not slept)

| # | Scenario | Card | Model time | `authorization_result` | `anomaly_result` | `final_action` | `alert` |
|---|---|---|---|---|---|---|---|
| 1 | Normal authorized | card-allow | 10:00 (daytime) | ALLOW/ROLE_ALLOW | NORMAL | **ALLOW** | false |
| 2 | Anomalous but authorized | card-allow | 03:00 (night) | ALLOW/ROLE_ALLOW | ANOMALOUS (UNUSUAL_ACCESS_TIME) | **ALLOW** | true |
| 3 | Unauthorized | card-deny | 03:00 (night, also anomalous) | DENY/NO_PERMISSION | ANOMALOUS (UNUSUAL_ACCESS_TIME) | **DENY** | true |

Scenario 3 is deliberately run at the *same* anomalous hour as scenario 2:
this is the direct proof of the required invariant — the anomaly
classification is reported honestly either way, but it never changes
`final_action`. A DENY stays a DENY.

## Reproduction

```bash
# (assumes the full system is already running — see Makefile `run-*` targets)

# scenario 1: ensure model time is in 07:00-19:00, then
curl -X POST http://localhost:8081/scenario/present -H 'Content-Type: application/json' \
  -d '{"reader_id":"reader-1","card_id":"card-allow","door_id":"door-1","direction":"PUBLIC_TO_EMPLOYEE"}'

# scenario 2: advance into the night window, then repeat the same presentation
curl -X POST http://localhost:8081/modeltime/advance -H 'Content-Type: application/json' -d '{"seconds": 61200}'
curl -X POST http://localhost:8081/scenario/present -H 'Content-Type: application/json' \
  -d '{"reader_id":"reader-1","card_id":"card-allow","door_id":"door-1","direction":"PUBLIC_TO_EMPLOYEE"}'

# scenario 3: same (still-night) model time, unauthorized card
curl -X POST http://localhost:8081/scenario/present -H 'Content-Type: application/json' \
  -d '{"reader_id":"reader-1","card_id":"card-deny","door_id":"door-1","direction":"PUBLIC_TO_EMPLOYEE"}'
```

Then inspect `data/evidence.jsonl`'s `decision` records, or
`http://localhost:8085/` (the dashboard), which shows the latest
decision's `final_action`, `authorization_reason`, and `anomaly_result`
together.

## Limitation, stated honestly

The repeated-attempt factor and the unusual-time factor are the only two
implemented; a third candidate (unusual destination/zone) is not, because
this project's configuration has exactly one controlled door — there is
nothing for a destination-unusualness factor to vary against, so adding
one now would be untestable, not genuinely evaluated. Thresholds (0.6 per
factor, 0.5 classification cutoff, 07:00–19:00 window, 5s repeat window)
are a defensible starting baseline, not values derived from a real
evaluation data set — none exists for this project.
