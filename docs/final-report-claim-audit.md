# Final Report — Claim Audit

An independent, skeptical read of `report/final-report.tex` against the
implementation, the evidence files, and the project's other documents.
The question asked of every important claim: is this supported by
implementation, supported by test/evaluation evidence, worded no
stronger than the evidence warrants, and consistent with every other
document that touches the same fact? Nothing here was softened because
the author and the reviewer are the same session — findings are reported
as found. No report edits were made during this pass.

**Severity key**: **BLOCKER** — an internal contradiction or unverifiable
claim that a careful examiner would find immediately and that undermines
the report's central "evidence, not assertion" pitch. **IMPORTANT** —
weakens the report's credibility or precision if an examiner probes it,
but doesn't contradict the system's actual behavior. **MINOR** —
technically imprecise wording; low risk, cheap to tighten.

---

## BLOCKER

### B1. §3 Requirements table: REQ-AUTH row arithmetic is wrong

Report line 225: `REQ-AUTH-001--008 | ... | 6 PASS, 2 PARTIAL`. That
sums to 8 with zero UNIMPLEMENTED. The actual breakdown, per
`docs/traceability-matrix.md` Part B, is **5 PASS, 2 PARTIAL, 1
UNIMPLEMENTED** (REQ-AUTH-005, time-of-day/day-of-week permission
windows, is UNIMPLEMENTED — `PermissionsFor` evaluates no day/time
field at all). This is not a stray inconsistency with an external
document: the report's **own §13 risk list**, four lines below the
table, names "REQ-AUTH-005 time windows" explicitly as one of the seven
unimplemented requirements. The table and the prose fifteen pages later
flatly disagree about whether REQ-AUTH-005 exists as a gap. An examiner
cross-checking the very first evidentiary table against the document's
own later claims — exactly the kind of diligence this report explicitly
invites ("evidence, not assertion") — finds a contradiction in under two
minutes.

The *grand totals* elsewhere in the report (41 PASS / 6 PARTIAL / 7
UNIMPLEMENTED, §3 prose and §14) are correct and match
`docs/traceability-matrix.md` exactly — only the REQ-AUTH **row** is
wrong. This makes it read as a transcription slip, not a comprehension
gap, but it is still a factual error sitting in the report's first table.

**Fix**: change the table cell to `5 PASS, 2 PARTIAL, 1 UNIMPLEMENTED`.
(See `docs/final-report-edit-list.md` item P0-1.)

---

## IMPORTANT

### I1. Decision-restart recovery time: two different numbers for the same claim, across documents

- `report/final-report.tex` §11.3 and `docs/evaluation-results.md` §2:
  **"min 10.83 ms, mean 18.8 ms, max 32.26 ms"** — from
  `evidence/final-verification/10-recovery-time-measurements.log`, a
  fresh 10-run capture taken during the final-verification pass.
- `docs/evaluation-plan.md` §3 ("Result"), `docs/traceability-matrix.md`
  (REQ-RES-004 row), and `docs/final-report-gap-analysis.md` §4 all
  still say **"11–34 ms"** — an earlier 10-run capture from a prior
  session.

Both numbers are real, both are honestly measured (not fabricated), and
they overlap substantially (10.83–32.26 vs. 11–34 is the same
phenomenon measured twice, close in magnitude) — this is not a
fabrication finding. But a report whose central pitch is "every number
is read from a committed evidence file, not asserted" should not have
three of its own supporting documents quoting a different specific
number for the same metric than the report itself quotes. An examiner
who opens `docs/traceability-matrix.md` (which the report explicitly
tells them to do, repeatedly, as "the single authoritative
requirement-status table") will see "11–34 ms" immediately next to
REQ-RES-004, while the report says "10.83–32.26 ms, mean 18.8 ms" — a
direct, checkable mismatch between a document the report calls
authoritative and the report's own evaluation section.

**Fix**: update the three stale citations to the fresh numbers (or, more
robustly, cite the specific run/log file so each document's number is
self-explanatorily tied to one measurement event rather than presented
as a single immutable constant). See edit-list P0-2.

### I2. "Changes since the proposal" cites a document not present in, or verifiable from, the repository

Report §1: *"Changes since the proposal. The system converged on a
materially smaller physical footprint than a typical multi-room
ventilation use case would require..."* — this describes specific,
concrete differences from a Week-2 project proposal. No proposal
document for *this* project exists anywhere in the repository (a
repo-wide search for `proposal` finds only the course's generic
`proposal_example`/`proposal_guide` templates under `docs/internal`,
which are illustrative course material, not this student's own
submission). The comparison may well be accurate — the student may have
a real submitted proposal to show at the oral exam — but **from the
repository alone, this claim is unverifiable**, and the report presents
it as a settled fact. A skeptical examiner's natural first move
("show me the proposal you're referring to") has no document to point
to.

**Fix**: either attach the actual proposal (or a link/reference to where
it was submitted, e.g. Canvas) to the repository, or soften the claim so
it doesn't assert a specific, unverifiable document comparison. See
edit-list P0-3.

### I3. "Nine containers" is used throughout, but `compose.yaml` defines ten services

`compose.yaml` has 10 top-level services: `buildsim`, `door-seed`,
`simulator`, `eventdata`, `decision`, `actuator`, `cardreader`,
`occupancysensor`, `doorstatesensor`, `dashboard`. The report's deployment
diagram caption (§9) says "nine containers," and §10's results line says
"all 9 containers reach running/healthy." This is **defensible** —
`door-seed` is a one-shot init container that is *supposed* to exit
(status `Exited 0`), not join the "running/healthy" set, and plain
`docker compose ps` (used throughout verification) correctly omits
exited one-shot containers from its default view — but the report never
states this reconciliation anywhere. A reader who actually opens
`compose.yaml` and counts ten service blocks, then reads "nine
containers" four times in the deployment section, has a legitimate
question the report doesn't answer: is it nine or ten, and why the
discrepancy? The explanation is good and already written (§9's own
prose: "plus a one-shot `door-seed` init container") — it is simply
never connected to the "nine" count used elsewhere.

**Fix**: state once, explicitly, near the first use of "nine
containers": "ten Compose services total; nine are expected to reach a
steady running/healthy state, the tenth (`door-seed`) is a one-shot init
step expected to exit." See edit-list P0-4.

### I4. Self-referential rubric-grading language inside the main report body

Report §9, lines 710–713: *"This is named here deliberately, not only in
the risks section: it is an example of exactly the kind of 'demonstrate
and analyse behaviour under change or failure' the course asks for at
Dimension B, grade 4."* This is the report explicitly telling the
examiner what grade level a specific piece of evidence satisfies. This
is the *only* place in the main report body that does this (the rest of
the report states facts and lets the reader judge), which makes it stand
out rather than read as house style. Self-grading language inside the
evidentiary document itself risks reading as presumptuous or as trying
to pre-empt the examiner's own judgment — exactly the kind of thing a
grade-5 Dimension D self-assessment ("critical, not self-congratulatory")
should avoid. The *content* of the claim is accurate and well-evidenced;
it's the framing that's a liability. `docs/report-rubric-audit.md` is
the correct, already-existing home for exactly this kind of
rubric-mapped claim — it does not belong duplicated, unhedged, inside
the main report.

**Fix**: remove the "Dimension B, grade 4" clause from §9; let the
door-seed finding stand on its own evidence (it's strong enough to). See
edit-list P0-5.

### I5. §4.4's design-decision table doesn't distinguish evidence-backed reasoning from argued-from-principle reasoning

Of the six rows in the design-decisions table, only the REST-vs-broker
row has a load-bearing link to *measured* evidence (the stress
experiment, indirectly — via `internal/httpx`'s bounded timeouts being
part of why fault experiments were injectable at well-defined points).
The other five (one-process-vs-microservices Decision Service, Event/Data
on/off the control path, JSONL vs. a database, rule-based vs. ML
anomaly, Compose vs. Kubernetes) are each given a reasoned argument but
no comparative measurement — no alternative was ever built far enough to
produce a number against. This is not dishonest (the rubric's grade-4
bar explicitly accepts "justified against at least one reasonable
alternative" without requiring a measurement), but the report's own
Section 11 opens with "every number below is read from a committed
evidence file," and a reader moving from §11 back to §4.4 could
reasonably — incorrectly — assume the same evidentiary standard applies
there. It doesn't, and the report never says so.

**Fix**: a one-sentence note at the top of §4.4 clarifying that these are
reasoned justifications, not all measured comparisons, with the one
exception that is. See edit-list P1-1 (not P0 — this is a clarity gap,
not a factual error).

---

## MINOR

### M1. "Domain separation under observable evidence" is presented as the thread behind *every* design decision

Report §1: *"The guiding principle threaded through every design
decision is domain separation under observable evidence."* True for the
authorization/anomaly split and the Decision Service's internal module
boundary. Not really true for REST-vs-broker (driven by traffic shape
and operational simplicity), JSONL-vs-database (driven by write-pattern
simplicity and restart-survivability), or Compose-vs-Kubernetes (driven
by scale-appropriateness) — those are justified on their own, different
grounds in §4.4 itself. The word "every" overclaims thematic unity the
report's own decision table doesn't actually demonstrate.

**Fix**: narrow the claim to the decisions it actually describes
(authorization/anomaly, the Decision Service's module boundary), or
drop "every" for "several of the most important." See edit-list P2-1.

### M2. "Every timer in the codebase states which clock it uses in its own comment"

True for `relockTimeout` ("10-model-second"), `repeatedAttemptWindow`
("// model time"). Less literally true for `httpx.DefaultTimeout`,
`unlockWaitTimeout`, `unlockPollInterval`, `passageDwell` — each is
*unambiguously* wall-clock from context (a network timeout, a poll
interval, a sleep), but none of their doc comments contains the word
"wall-clock" explicitly. The underlying property (no timer is ever
ambiguous about which clock it measures) holds; the specific claim about
*comments* stating it is slightly stronger than literally true.

**Fix**: soften "states" to "is documented or unambiguous from its
network/poll/model-time purpose." See edit-list P2-2.

### M3. Stress-table rounding at concurrency 3000

`evidence/stress/summary.csv` records `throughput_rps=1686.50` at
concurrency 3000; the report table shows `1687`. Defensible rounding
(half-up), not an error, but a numerically pedantic examiner could ask
why a `.50` value rounded up rather than to even. Lowest-priority item
in this audit — included for completeness, not because it needs fixing.

---

## What was checked and found to hold up (for balance, not just failures)

- The stress-test table's every other number (concurrency 1, 10, 50,
  100, 3000 throughput/p95/error-rate) matches
  `evidence/stress/summary.csv` exactly.
- The anomaly-policy table matches both `evidence/anomaly/README.md`
  and the live Docker re-capture in
  `evidence/final-verification/05-decisions-check.log` exactly.
- The latency numbers (n=200, p50/p95/max for both metrics) match
  `evidence/latency/results.json` exactly, to the millisecond digit
  reported.
- The claim that "every inter-service call shares `http.DefaultClient`"
  is not just true but actually *understates* the scope: reading
  `internal/buildsim/http_client.go` confirms BuildSim calls (Actuator,
  Simulator, DoorStateSensor → BuildSim) go through the same
  `internal/httpx` package and therefore the same `http.DefaultClient`
  too — the shared-pool exposure is wider than the report's wording
  implies, not narrower. (A safe, true strengthening opportunity, not a
  correction — see edit-list P2-3.)
- The 5-role, 8-person population claim (§6: "employee, facilities,
  manager, visitor, student") matches `config/population.json` and
  `config/system-config.json` exactly — 5 roles defined, 5 role labels
  used, 3 with ALLOW permissions (employee/facilities/manager), 2
  without (visitor/student).
- The `normal-day` seed-42 reproducibility claim ("consistently
  selecting `[card-henry, card-frank, card-dave, card-erin,
  card-carol]`") is independently confirmed by at least three separate
  real runs across this project's sessions, not just the unit test — a
  genuinely supported claim, not an inference from a single observation.
- Every cited test function name (`TestDecide_Unauthorized_
  DeniedRegardlessOfAnomaly`, `TestAuthorize_Deterministic`, etc.) exists
  verbatim in the codebase.
- `docs/data-contracts.md` does contain a concrete JSON example payload
  for every endpoint the report cites as an example, matching the
  report's "endpoint, method, schema, and an example payload for each"
  claim precisely.
- BuildSim's wholesale-replace PUT semantics claim is traceable to an
  actual source-code comment in `internal/buildsim/http_client.go`
  citing BuildSim's own `pkg/store/memory.go` — not an assumption dressed
  up as a fact.

## Summary count

**1 BLOCKER, 5 IMPORTANT, 3 MINOR.**
