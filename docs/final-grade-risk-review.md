# Final Grade Risk Review

A fresh re-evaluation of Dimensions A–D, done independently of
`docs/report-rubric-audit.md`'s own conclusions (that document was
written by the same session that wrote the report; this one was written
afterward, treating the report, the traceability matrix, and the
evidence directories as the only ground truth, and the earlier audit as
just another document to be skeptical of, not a starting point to
confirm). Where this review agrees with the earlier audit, that is
stated as agreement reached independently, not inherited. Where it
disagrees or adds a sharper edge, that is flagged explicitly.

This document does not award a grade. It states confidence levels and
the single biggest capping risk per dimension, so the examiner's actual
judgment is better informed, not pre-empted.

---

## Dimension A — Architecture & Design

**Strongest evidence**: The design-decisions table (report §4.4) names a
real rejected alternative for all six major decisions, each with a
specific, non-generic reason tied to this project's own constraints (not
textbook boilerplate). The C4 context/container/component diagrams exist,
are legible, and are narrated with specific component-to-responsibility
mapping, not just labeled boxes. Interface precision
(`docs/data-contracts.md`) is genuinely strong — every endpoint has a
concrete JSON example, verified in this review to actually exist for
every endpoint the report cites.

**Weakest evidence**: Of six architectural decisions, only one
(REST-with-bounded-timeouts) has any evidence that traces back to a
measurement; the other five are reasoned, not compared. The grade-5
"evolution to larger deployments" treatment (report §13's first bullet)
is **one paragraph** naming three specific scaling pressure points
(evidence-file partitioning, per-door state contention, BuildSim's
wholesale-replace PUT) — concrete and specific, which is good, but it is
assertion-depth, not analysis-depth: none of the three pressure points is
quantified (no back-of-envelope calculation, no "at N doors, X would
need to change by Y"). It reads as "we know what would need to change,"
not "we have reasoned carefully about how and when."

**Likely examiner interpretation**: An examiner reading charitably will
credit the specificity of the scaling discussion (it names real
mechanisms, not generic "it would need to scale better") and the
genuine, non-boilerplate alternatives table. An examiner reading
skeptically will note that "critically evaluated... using evidence and
engineering reasoning" (the rubric's own grade-5 language) is mostly
satisfied by the "reasoning" half, thinly by the "evidence" half, and
ask directly: *which of these six decisions would you actually revisit
if you measured the alternative?* — a question the report currently has
no strong answer to, because no alternative was measured.

**Grade confidence**: Grade 3 — high confidence, fully met. Grade 4 —
high confidence, fully met (NFR-linked decisions, behavioural models,
precise interfaces are all genuinely present, not just claimed). Grade 5
— moderate confidence at best; defensible as "attempted and partially
achieved" rather than "clearly demonstrated."

**Single biggest reason this could be capped below 5**: The
evidence-vs-reasoning asymmetry across the six decisions, combined with
the scaling discussion being assertion-depth rather than analysis-depth.
An examiner is unlikely to doubt the *decisions themselves* were good
ones — the risk is entirely about whether the *justification standard*
reaches "critically evaluated... using evidence," which for five of six
decisions it does not.

**How to discuss this intelligently without building anything new**: Be
explicit, unprompted, about which decisions are evidence-backed and
which are reasoned-from-principle — doing this voluntarily (rather than
waiting to be caught not having a measurement) is itself a Dimension-A
grade-5 move, because it's exactly the kind of "assumptions, limitations,
trade-offs" self-awareness the rubric asks for. Naming *which single
alternative* would be most informative to actually measure (this review
and the oral-defense-prep doc both suggest the microservice-per-module
Decision Service variant) demonstrates the reasoning is principled, not
hand-waved, even without building it.

---

## Dimension B — Systems Integration & the CPS Loop

**Strongest evidence**: Two genuinely independent, root-caused,
confirmed-not-assumed findings exist: the HTTP connection-pool
exhaustion (stress experiment) and the `door-seed`-vs-bulk-restart DNS
race (found during this project's own final Docker verification, not
planted for the report). Both are real engineering discoveries with a
specific mechanism named and a specific piece of evidence (a log line)
backing the mechanism, not a plausible-sounding guess.

**Weakest evidence**: Of the rubric's four named grade-4 failure
examples (process crashes, unavailable services, delayed messages,
invalid sensor data), only two have any test coverage at all. "Delayed
messages" and "invalid sensor data" are not just under-tested — there is
*no mechanism in the codebase* to even produce either condition
deliberately, so there's no clear path to generating that evidence
without building something new (out of scope for this review to
recommend, but worth being precise about the gap's shape: it is not
"we tested it a little," it is "we have never produced this condition at
all").

**Likely examiner interpretation**: This review's independent judgment:
an examiner could reasonably push back on treating the `door-seed` race
as evidence of "emergent/system-level... behaviour" in the rubric's
grade-5 sense. The rubric's own language is about *runtime* behavior
under load/failure (cascading failures, feedback effects, resource
contention, conflicting decisions) — the door-seed finding is a
**startup-orchestration** race in Docker Compose's dependency semantics,
discovered and fixed in minutes once found. It is genuinely a systems-
level finding (an interaction between two architectural choices:
`depends_on` semantics and BuildSim's statelessness) and it is honestly
reported with a real fix and reverification — but a tough examiner could
characterize it as "good operational diligence," not "deep systems
reasoning about emergent behavior," and the report should not lean on it
as if it were the latter. The connection-pool finding is a much cleaner
fit for the rubric's grade-5 language (it is precisely "resource
contention... communication bottlenecks").

**Grade confidence**: Grade 3 — high confidence, fully met (the loop is
real, traced, demonstrated three ways: fake client, real BuildSim,
Docker). Grade 4 — moderate-high confidence; the two-of-four
failure-example coverage is a real, specific gap an examiner will likely
notice, but what *is* tested is tested rigorously (automated + real-
process + Docker, three ways, for both covered examples). Grade 5 —
moderate confidence; strong on the connection-pool finding alone, but
that single finding may need to carry the whole grade-5 case if the
door-seed race is discounted as "ops hygiene" by a skeptical reader.

**Single biggest reason this could be capped below 5**: If an examiner
discounts the door-seed finding's weight (per the interpretation risk
above), Dimension B's grade-5 case rests on one finding, not two — still
plausibly sufficient on its own merits, but a thinner case than the
report currently presents it as.

**How to discuss this intelligently**: Don't contest a skeptical
examiner's framing of the door-seed race if they raise it — agree that
it's narrower in scope than the connection-pool finding, and pivot
directly to the connection-pool finding's own depth (the benchmark-
artifact elimination step, the exact log-line root cause, the ~4x
internal fan-out mechanism) as the stronger standalone case. Naming the
two-of-four failure-example gap unprompted (delayed messages, invalid
sensor data never produced at all, not just under-tested) is a stronger
oral-exam move than waiting for the examiner to find it.

---

## Dimension C — Autonomous Services & Data Engineering

**Strongest evidence**: The autonomous component itself is well-built
and well-evaluated: authorization is structurally deterministic and
proven so by test; anomaly detection is a genuine, if simple, two-factor
scorer with both positive and negative test cases; the non-override
invariant (REQ-DEC-003) is proven structurally, not just by convention —
the strongest single piece of evidence in this whole dimension, because
it's exactly the kind of "interaction between autonomous decision-making
and X" reasoning the rubric's grade-5 language wants, just applied to
authorization rather than to data engineering.

**Weakest evidence**: This review's independent read agrees with, and
sharpens, the earlier audit's conclusion: the data pipeline does **zero**
of the three data-quality behaviors the rubric's grade-5 language names
by example (missing, delayed, contradictory, or corrupted data,
"identified and handled appropriately"). This is not a partial-credit
situation — duplicate detection, late/out-of-order detection, and
retained/counted malformed-data handling are each entirely absent, not
thin. The one place this project *does* reason about a data-quality
interaction (§C6 of the adversarial-questions section: a duplicate
presentation would falsely trigger the anomaly scorer's repeated-attempt
factor) is a *prediction*, reasoned correctly from the code, not a
demonstrated or tested behavior.

**Likely examiner interpretation**: This is the one dimension where this
review expects very little interpretive disagreement among examiners.
The rubric's grade-5 wording for Dimension C is unusually concrete and
checklist-like compared to the other three dimensions' more
interpretable language ("cascading failures," "emergent behaviour" allow
some reading latitude; "identified and handled appropriately" does not).
An examiner applying the rubric as written will not find grade-5
evidence here, because the described behavior simply isn't in the
system. The project's own documents already say this plainly
(`docs/traceability-matrix.md`, report §13) — which is itself a
mitigating factor for Dimension D ("honest critique"), but does not
change Dimension C's own standing.

**Grade confidence**: Grade 3 — high confidence, fully met. Grade 4 —
high confidence, fully met (pipeline exists, stores, serves; autonomous
component evaluated with real metrics; compared against a reasonable
alternative). Grade 5 — **low confidence**. This is the one dimension in
this review where "capped below 5" is closer to "very likely" than "a
risk to be aware of."

**Single biggest reason this is capped below 5**: Not a matter of
interpretation or framing, unlike A and B above — the rubric asks for a
specific, concrete capability (data-quality identification and handling)
that this system does not have, in any form, anywhere in the pipeline.

**How to discuss this intelligently without building anything new**:
This is the one place in the whole exam where the strongest available
move is not reframing or emphasis — it's precision and completeness.
Three things make this gap read as mature critical thinking rather than
an admission of incompleteness: (1) name the gap's exact shape without
being asked (zero duplicate detection, zero late/ordering detection,
malformed data rejected-but-not-retained-or-counted — not "data quality
isn't fully handled," the specific missing mechanisms); (2) show the
*predicted* interaction this review surfaced (§C6 of the adversarial
questions: duplicate presentations would corrupt the anomaly scorer's
repeated-attempt factor today) — this demonstrates the system's
designer understands *where* the gap would bite the autonomous
component specifically, which is a real piece of Dimension-C reasoning
even without building the fix; (3) explicitly separate this gap from the
authorization/anomaly separation (REQ-DEC-002/003), which *is* strong —
don't let the one strength imply coverage of the other weakness; keep
them visibly distinct in any answer.

---

## Dimension D — Critical Evaluation

**Strongest evidence**: The stress/breaking-point experiment is real,
methodologically careful (ruled out a benchmark-tool artifact before
trusting the result), and surfaced a genuinely new finding (the
Decision→Actuator missing-outcome gap) that was not fabricated for the
report — it emerged from the experiment and is reported honestly. The
risks section (§13) is long, specific, and each item is tied to a named
requirement ID or evidence file, not generic hedging language.

**Weakest evidence, found by this review specifically**: Two things a
purely internal self-assessment would likely miss, because they require
comparing documents against each other, not just reading the report in
isolation: (1) the recovery-time number inconsistency
(`docs/final-report-claim-audit.md` I1) — three sibling documents still
cite an older measurement than the report itself does, which is exactly
the kind of inconsistency that undercuts "evidence, not assertion" if an
examiner cross-references; (2) sample-size honesty varies by
measurement — n=200 for latency is solid, n=10 for recovery time is
adequate for the claim actually made (a range, not a percentile), but
the stress sweep is **one run per concurrency level**, which is the
thinnest evidentiary base in the whole evaluation section and is not
flagged as such anywhere in the report (see adversarial question D5).

**Likely examiner interpretation**: Favorable: the breadth and honesty
of the evaluation (nominal, latency, failure/recovery, anomaly, stress —
five distinct evaluation angles, each with real numbers) is hard to
dispute as satisfying grade 4 comfortably and grade 5's "systematic
exploration of limits" substantially. Unfavorable: a numerically careful
examiner who notices the stress sweep is single-run, or who
cross-references the recovery-time number against `docs/traceability-
matrix.md` and finds it stale, will read this as the evaluation being
*presented* with more precision/consistency than it actually has —
not a fabrication, but a polish gap that a grade-5 "critical evaluation"
exam should probably have caught internally before an external examiner
did.

**Grade confidence**: Grade 3 — high confidence, fully met. Grade 4 —
high confidence, fully met. Grade 5 — moderate-high confidence; the
stress experiment alone is a strong, genuine grade-5 anchor, but the two
polish gaps above (stale cross-document numbers, unflagged single-run
sampling) are exactly the kind of thing that should not exist in a
report whose central claim is rigorous self-cross-checking, and they
were found by this review, not pre-empted by the report itself.

**Single biggest reason this could be capped below 5**: Not the
*content* of the evaluation (which is genuinely strong) but its
*internal consistency and sampling transparency* — the claim-audit
findings (I1, D5-equivalent) are small individually but are exactly the
category of thing Dimension D's own grade-5 language asks the project to
catch about itself ("honest critique... supported by evidence from
testing and experimentation," which implies the critique should extend
to the evaluation's own methodology, not just the system's behavior).

**How to discuss this intelligently**: Acknowledge the single-run stress
sampling directly if asked about sample sizes (adversarial question D5
already has the honest answer prepared) — the qualitative finding (a
real breaking point exists, root-caused) is very likely robust to
re-measurement even though the exact numbers might shift run to run;
say this distinction plainly rather than defending the precision of
numbers that were only measured once.

---

## Cross-dimension observation (new to this review, not in the earlier audit)

The earlier `docs/report-rubric-audit.md` treats each dimension
independently. This review adds one observation that only emerges from
comparing them: **Dimension C's gap and Dimension D's strength are the
same underlying fact, viewed from two directions.** The absence of
data-quality handling is a *weakness* when judged as "does the system
do this" (Dimension C), and simultaneously *evidence of honest
self-assessment* when judged as "does the critique identify real
weaknesses" (Dimension D) — the project gets credit in D for naming
exactly the gap that costs it in C. This is not a contradiction to
resolve; it's worth understanding and saying explicitly if an examiner
asks "doesn't this gap hurt your grade?" — the honest answer is "it
caps Dimension C, and the same honesty about it is part of what
Dimension D is asking to see."

## Summary table

| Dimension | Grade 3 | Grade 4 | Grade 5 | Biggest single cap risk |
|---|---|---|---|---|
| A — Architecture | High confidence | High confidence | Moderate | Evidence-vs-reasoning asymmetry; scaling discussion is assertion-depth |
| B — Systems/CPS loop | High confidence | Moderate-high | Moderate | Only 2 of 4 rubric failure-examples tested; door-seed finding's weight is contestable |
| C — Autonomous/Data | High confidence | High confidence | **Low** | Data-quality handling is entirely absent, not partial — a concrete rubric requirement unmet |
| D — Critical evaluation | High confidence | High confidence | Moderate-high | Internal numeric inconsistency + unflagged single-run stress sampling |
