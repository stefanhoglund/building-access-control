# Final Report — Rubric Audit

A conservative, section-by-section audit of `report/final-report.tex`
against the course's official grading criteria (Stage 2: four dimensions,
A–D, each at levels 3/4/5; grades are determined holistically, and a
significant weakness in any one dimension can cap the overall grade even
where others are strong). This document does not award a grade — that is
the examiner's call — it states, per dimension, what evidence currently
exists for each level and names the specific weakness that would cap it,
so nothing here is asserted without a report section or evidence file
backing it.

**Stage 1 (mandatory pass/fail) is verified separately and in full in
`docs/traceability-matrix.md` Part A — all nine items PASS.** This
document covers only Stage 2.

## Dimension A — Architecture & Design

*Guiding question: is the architecture clearly described, well justified,
and appropriate for the problem?*

**Grade 3 — supported.** C4 context and container diagrams (report
§4.1–4.2), a requirements table with IDs (§3, backed by the full
`docs/traceability-matrix.md`), and interface contracts
(§5, `docs/data-contracts.md`). Component responsibilities are stated
explicitly (§4.2 narrative). Six major design decisions are each given at
least one named, rejected alternative with a reason (§4.4 table).

**Grade 4 — supported.** Several decisions are explicitly tied to a
named non-functional property: REST-not-MQTT to communication simplicity
and bounded-timeout precision (§4.4, §5), JSONL-not-a-database to
restart-survivability (§4.4, REQ-DATA-002), the rule-based anomaly
baseline to testability of the core safety invariant (§4.4, §7.1).
Behavioural models exist and are narrated in detail: the dynamic/sequence
diagram (§8.1) and the door-control state diagram (§8.2). Interfaces are
specified precisely in `docs/data-contracts.md` — every endpoint, schema,
and example payload — to a level that plausibly supports independent
re-implementation.

**Grade 5 — partially supported; the specific gap.** The architecture
*is* critically evaluated (§13, an 11-item honest limitations list, not a
token one) and the evolution question is directly addressed (§13's first
bullet: what changes at 100 doors or multiple buildings, named
component-by-component, not just asserted as "it would scale"). Where
this falls short of a clean grade-5 showing: the rubric asks alternatives
to be "discussed and compared **using evidence and engineering
reasoning**" — the §4.4 table gives engineering reasoning for every
alternative, but only one of the six decisions (the connection-pool/REST
choice, evidenced by the stress experiment, §11.5) is backed by measured
evidence rather than argument from principle. The other five alternatives
(one-process-per-module Decision Service, direct sensor-to-decision
routing, a message broker, a database, a learned anomaly model) were
reasoned about, not empirically compared — no microservice-decomposed
variant or MQTT variant was ever built to measure against. **This is the
dimension's capping weakness**: strong reasoning, evidence-backed in one
case, argued-from-principle in the rest.

## Dimension B — Systems Integration & the CPS Loop

*Guiding question: do you reason about the system as a whole, not just
isolated components?*

**Grade 3 — supported.** The full sense→decide→act→physical-effect→sense
loop is demonstrated end-to-end (§8.1, directly tested by
`test/e2e/passage_test.go` and reproduced against real BuildSim and
Docker). Nine independent processes communicate over documented
interfaces (§4.2, §5); §8.1's narrative traces one access attempt through
every one of them.

**Grade 4 — supported, with one named gap.** Service boundaries,
communication patterns, and responsibilities are justified (§4.4).
Behavior under failure is demonstrated and evaluated for two of the
rubric's four named failure examples: **process crashes** (Decision
Service killed, §11.3) and **unavailable services** (BuildSim/Actuator
killed, §11.3). The other two named examples — **delayed messages** and
**invalid sensor data** — have no corresponding experiment: REQ-DATA-005
(late/out-of-order detection) and REQ-DATA-006 (malformed-observation
handling beyond a bare HTTP 400) are UNIMPLEMENTED/PARTIAL
(`docs/traceability-matrix.md`), stated as such in §13, not glossed over.
Recovery mechanisms that *do* exist are measured, not just claimed
(§11.3: 10.83–32.26 ms recovery, 10 fresh runs).

**Grade 5 — supported; this is the report's strongest dimension.** Two
independent, confirmed-root-cause, system-level findings exist, not one:
(1) the HTTP connection-pool exhaustion under concurrent load (§11.5) —
a genuine communication-bottleneck analysis with the root cause isolated
to a specific default (`http.DefaultClient`'s 2-idle-connections-per-host
limit) and confirmed by ruling out a benchmark-tool artifact first; and
(2) the `door-seed` vs. bulk-restart DNS race found during Docker
verification (§9) — a genuine emergent interaction between Compose's
`depends_on` semantics and a multi-container restart, reproduced twice,
root-caused from the actual error message, and fixed. Both explicitly
relate the limit back to an architectural decision (shared
`http.DefaultClient`; `restart` vs. `up` dependency semantics), which is
exactly what the rubric asks for at this level. The single-pending-
per-door overlap limitation (§13) is a third, smaller example of the same
kind of system-level reasoning, reported as a limitation rather than
deeply investigated as a case study — supporting evidence, not the
dimension's main claim.

## Dimension C — Autonomous Services & Data Engineering

*Guiding question: does the system make autonomous decisions supported by
a well-designed data pipeline?*

**Grade 3 — supported.** The Decision Service makes autonomous decisions
from sensor data (§7.1); data flows through a defined pipeline
(Event/Data Service, §7.2), never sensor-to-decision directly; the
rule-based approach is explained and justified against a learned-model
alternative (§4.4, §7.1).

**Grade 4 — supported.** The pipeline collects, stores, and serves data
for monitoring (dashboard, §12), analysis, and evaluation (every
`evidence/*` experiment directory, §11). The autonomous component is
evaluated with real metrics and evidence (§11.4's anomaly table, §11.2's
latency numbers). The rule-based approach is compared against a learned
alternative with named strengths/limitations (§4.4, §13).

**Grade 5 — not supported; this is the report's clearest capping
weakness.** The rubric's own wording is specific: "data quality issues
such as missing, delayed, contradictory, or corrupted data are
**identified and handled appropriately**." This project's data pipeline
does neither duplicate detection (REQ-DATA-004), late/out-of-order
detection (REQ-DATA-005), nor meaningful malformed-data handling beyond a
bare HTTP 400 with no retention or counting (REQ-DATA-006 PARTIAL) — all
stated plainly as UNIMPLEMENTED/PARTIAL in §13 and
`docs/traceability-matrix.md`, not hidden. The interaction between
anomaly detection and authorization *is* analyzed (§7.1, §11.4, REQ-DEC-
002/003 structurally proven) — but that is an autonomy/policy interaction,
not the data-quality interaction this grade level asks for. **Until
duplicate/late/malformed handling exists and is evaluated, Dimension C
should be read as capped at grade 4, not 5** — this is named here
precisely so it is not mistaken for an oversight in the report itself.

## Dimension D — Critical Evaluation

*Guiding question: what evidence do you have that the system works, and
that it does not?*

**Grade 3 — supported.** §10–11 describe what was tested, how, and what
the results show, for both individual components (unit tests) and
end-to-end behavior (e2e/integration/scenario tests, Docker verification).

**Grade 4 — supported.** Quantitative evaluation against a stated
requirement exists (REQ-PERF-001, §11.2, measured *before* the threshold
was chosen — stated explicitly, not after-the-fact tuning). Metrics
measured: decision latency, actuation-confirmation latency, recovery
time, throughput, error rate, anomaly classification correctness.
Fault-injection experiments (Decision kill, Actuator/BuildSim kill, §11.3)
are each connected back to an architectural property (bounded timeouts,
fail-closed-by-construction, the actuator's explicit verify-before-
confirm step).

**Grade 5 — supported; the report's second-strongest dimension, with one
honest caveat.** A genuine stress/breaking-point experiment exists
(§11.5), not a nominal-load check dressed up as one: concurrency swept
across three orders of magnitude, a real collapse to zero throughput
found, root-caused (not assumed) by first ruling out a benchmark-tool
artifact and confirming the ceiling did not move after fixing it. §13 is
an 11-item, evidence-backed critique — including a finding (the
Decision→Actuator missing-outcome gap) that the stress experiment itself
surfaced and that was not previously known, which is precisely the kind
of "evidence about limitations driving further reasoning" the rubric
describes, not a list of hypothetical weaknesses. **Caveat**: the
rubric's grade-5 list also names resource-consumption, energy, and
sustainability trade-offs; this report does not discuss any of these —
honestly, because a single-laptop building-access prototype has no
natural resource/energy story to tell, not because it was overlooked. An
examiner could reasonably treat this as a minor, use-case-appropriate
omission rather than a capping weakness, but it is named here so that
judgment is the examiner's to make, not asserted away.

## Holistic read (not a grade — an honest synthesis)

Dimensions B and D are both strongly supported at grade 5, each by more
than one independent, root-caused, evidence-backed finding. Dimension A
is solid at grade 4 with a specific, named grade-5 gap (reasoning over
alternatives is strong but mostly not evidence-based). Dimension C has a
clear, specific grade-5 gap: the data pipeline does not yet identify or
handle the data-quality issues the rubric asks for at that level, even
though the autonomous component itself is well-evaluated. Per the
course's own grading philosophy ("significant weaknesses in any dimension
may limit the final grade, even when performance in other dimensions is
strong"), **Dimension C's gap is the one most likely to cap the overall
grade below 5**, regardless of how strong B and D are. This is stated
plainly, not softened, because the course's own rubric asks for exactly
this kind of honest self-assessment at Dimension D grade 5 — and because
`docs/traceability-matrix.md` and report §13 already say the same thing
in different words. Nothing in this audit should be read as the report
silently claiming a grade-5 standing it has not earned on Dimension C.
