# Final Report — Edit List

Every edit below traces to a finding in `docs/final-report-claim-audit.md`
or `docs/final-grade-risk-review.md`. Nothing here inflates a claim or
adds invented evidence — every suggested edit either (a) fixes a factual
error, (b) adds a precision/hedge the evidence already supports but the
current wording doesn't express, or (c) removes a claim stronger than
the evidence warrants. `report/final-report.tex` was **not** modified
during this review pass; these are recommendations for the next edit
pass.

**P0** = needed before submission (factual error or unverifiable claim).
**P1** = high-value improvement (precision/defensibility, no new content
needed). **P2** = optional polish.

---

## P0 — needed before submission

### P0-1. Fix the REQ-AUTH row arithmetic error
**Section**: §3 Requirements, the summary table (around line 225).
**Current**: `REQ-AUTH-001--008 & ... & 6 PASS, 2 PARTIAL \\`
**Change to**: `REQ-AUTH-001--008 & ... & 5 PASS, 2 PARTIAL, 1 UNIMPLEMENTED \\`
**Why**: Contradicts the report's own §13 (which names REQ-AUTH-005 as
unimplemented) and `docs/traceability-matrix.md`. See claim-audit B1.

### P0-2. Reconcile the decision-restart recovery-time number across documents
**Section**: Primarily `docs/evaluation-plan.md` §3 ("Result"),
`docs/traceability-matrix.md` (REQ-RES-004 row), and
`docs/final-report-gap-analysis.md` §4 — not `report/final-report.tex`
itself, which already has the correct fresh number.
**Current** (three places): `11–34 ms (automated, confound-free)`
**Change to**: `10.83–32.26 ms, mean 18.8 ms (automated, confound-free;
10 fresh runs captured during final verification,
evidence/final-verification/10-recovery-time-measurements.log)`
**Why**: Report §11.3 and `docs/evaluation-results.md` already cite the
fresh number; these three documents still cite an older run. The report
calls `docs/traceability-matrix.md` "the single authoritative
requirement-status table" — it should not disagree with the report on a
specific measured value. See claim-audit I1.

### P0-3. Resolve the unverifiable "changes since the proposal" claim
**Section**: §1 Summary, "Changes since the proposal" paragraph.
**Options, in order of preference**:
1. If a real Week-2 proposal was submitted for this project, add it to
   the repository (e.g. `docs/proposal.md` or a PDF) and cite it by
   filename in this paragraph, so the comparison is checkable.
2. If no such document exists or is available, reword to avoid implying
   a specific, checkable prior document: e.g. *"The system's scope
   settled on one controlled boundary rather than a multi-room physical
   simulation, trading simulated-physics breadth for distributed-systems
   depth: fail-closed behavior under..."* — i.e., describe the design
   choice on its own merits without framing it as a delta against an
   unverifiable artifact.
**Why**: No proposal document for this project exists anywhere in the
repository; the claim is unverifiable from the evidence the report
otherwise insists on. See claim-audit I2.

### P0-4. Reconcile "nine containers" with Compose's actual ten services
**Section**: §4.2 (container diagram caption, "eight independently
deployable project processes"), §9 Deployment (diagram caption "nine
containers" and the "Eight project processes, each its own container"
paragraph), §10 Test plan ("all 9 containers reach running/healthy").
**Add once, at the first use in §9** (suggested wording): *"Ten Compose
services are defined in total: the eight project processes, BuildSim,
and one one-shot `door-seed` init container. Nine are expected to reach
a steady running/healthy state; `door-seed` is expected to exit (status
`0`) once door-1 is seeded, which is why `docker compose ps`'s default
view — and this report's 'nine containers' figure — excludes it."*
Then the existing "nine containers" uses elsewhere can stand, now that
the reconciliation exists once, clearly, near the first use.
**Why**: `compose.yaml` defines 10 services; an examiner who counts will
find a discrepancy the report never explains. See claim-audit I3.

### P0-5. Remove the self-graded rubric-dimension claim from the report body
**Section**: §9 Deployment, end of the door-seed paragraph.
**Current**: *"This is named here deliberately, not only in the risks
section: it is an example of exactly the kind of 'demonstrate and
analyse behaviour under change or failure' the course asks for at
Dimension B, grade 4."*
**Change to**: *"This is named here deliberately, not only in the risks
section, because it is exactly the kind of change-and-failure behavior
worth demonstrating with evidence rather than asserting."* (or simply
delete the sentence — the preceding three sentences of evidence already
make the point without it)
**Why**: The main report should present evidence and let the examiner
judge; explicit grade/dimension self-labeling belongs only in
`docs/report-rubric-audit.md`, which exists for exactly this purpose.
See claim-audit I4.

---

## P1 — high-value improvements

### P1-1. Flag which design decisions are evidence-backed vs. reasoned-from-principle
**Section**: §4.4, one sentence before the design-decisions table.
**Suggested addition**: *"Each decision below is justified against a
named alternative; only the REST-vs-broker row traces to a direct
measurement (the stress experiment, §11.5) — the other five are
reasoned from this project's own constraints, not from a built and
measured comparison."*
**Why**: §11 opens with "every number below is read from a committed
evidence file," which could lead a reader to assume the same evidentiary
standard applies to §4.4. It mostly doesn't, and the report should say so
itself rather than leave it for a reviewer to notice. See claim-audit I5
and grade-risk-review Dimension A.

### P1-2. Flag the stress sweep's single-run sampling explicitly
**Section**: §11.5, after the results table or in the root-cause
paragraph.
**Suggested addition**: *"Each concurrency level was swept once, for 2
seconds; this is sufficient to establish the qualitative finding (a
sharp collapse between 50 and 100 concurrency, caused by a confirmed,
reproducible root cause) but the specific throughput/latency figures at
each level should be read as single-run measurements of an inherently
noisy phenomenon, not high-confidence point estimates."*
**Why**: This is the thinnest sample size in the evaluation section and
is not flagged anywhere; a Dimension-D grade-5 "critical evaluation"
should surface this about its own methodology. See grade-risk-review
Dimension D and adversarial question D5.

### P1-3. Name the orphaned-unlock-on-restart scenario in §13
**Section**: §13 Risks, as a new bullet (or folded into the existing
"single-pending-per-door" bullet).
**Suggested addition**: *"If the Decision Service dies after issuing an
UNLOCK but before the door is observed OPEN, the restarted process has
no carried-over state and does not know this door has an outstanding
unlock to watch — nothing in the current design re-derives this
condition from BuildSim's own state on startup. Not exercised by any
test; found by this project's own adversarial review, not by an external
examiner."*
**Why**: A real, previously-unnamed gap, surfaced during this review
(`docs/oral-defense-prep.md`, Adversarial Examiner Questions B2). Naming
it in the report itself — not just in oral-defense prep — is a stronger
Dimension D move than waiting to be asked. Optional whether this becomes
P0 or P1; classified P1 because it doesn't contradict anything already
written, it adds a genuinely new, voluntarily-disclosed limitation.

### P1-4. Separate REQ-DEC-002/003 (authorization/anomaly independence) from data-quality handling explicitly in §13
**Section**: §13, near the existing anomaly-baseline bullet.
**Suggested addition**: one sentence making explicit that the
authorization/anomaly separation (strong, proven) is a *different*
property from data-quality handling (absent) — e.g. *"This structural
separation answers a different question than data-quality handling
below: it guarantees anomaly cannot override authorization, not that bad
input data is caught before reaching either signal."*
**Why**: Grade-risk-review flags the risk of these two being
conflated by a reader skimming for "Dimension C strengths." Keeping them
visibly distinct in the report itself, not just in oral-defense prep,
closes that reading before it starts. Directly derived from claim-audit/
grade-risk findings, not new content.

---

## P2 — optional polish

### P2-1. Narrow the "domain separation under observable evidence" claim
**Section**: §1 Summary.
**Current**: *"The guiding principle threaded through every design
decision is domain separation under observable evidence."*
**Change to**: *"The guiding principle behind the system's two most
important internal boundaries — authorization vs. anomaly, and the
Decision Service's own module structure — is domain separation under
observable evidence."*
**Why**: "Every design decision" overclaims thematic unity the §4.4 table
itself doesn't demonstrate (REST-vs-broker, JSONL-vs-database, and
Compose-vs-Kubernetes are justified on other grounds). See claim-audit M1.

### P2-2. Soften the "every timer states which clock" claim
**Section**: §5 Interfaces, "Model time vs. wall-clock time" paragraph.
**Current**: *"every timer in the codebase states which clock it uses in
its own comment"*
**Change to**: *"every timer in the codebase is either explicitly
commented with which clock it uses, or unambiguous from its network/
poll/model-time purpose"*
**Why**: True in spirit, slightly stronger than literally true for a
couple of wall-clock timers whose comments don't use the word
"wall-clock" explicitly. See claim-audit M2.

### P2-3. Strengthen (not just correct) the shared-connection-pool claim
**Section**: §11.5, root-cause paragraph.
**Suggested addition**: *"This pool is shared even more widely than
inter-project-service traffic: `internal/buildsim`'s HTTP adapter also
uses `internal/httpx`, so calls to BuildSim itself (from the Actuator,
Simulator, and DoorStateSensor) compete for the same global
`http.DefaultClient` pool as every other inter-process call."*
**Why**: Verified true by reading `internal/buildsim/http_client.go`
during this review — the current wording ("every inter-service call")
is accurate but actually understates the scope; this is a safe
strengthening, not a new or unverified claim. See claim-audit, "what was
checked and found to hold up."

### P2-4. Add a one-line rounding note to the stress table
**Section**: §11.5, table footnote.
**Suggested addition**: *"Throughput rounded to the nearest whole
request/s; p95/error figures to two decimal places, from
evidence/stress/summary.csv."*
**Why**: Trivial, but removes even the possibility of a pedantic
rounding question (concurrency 3000's 1686.50 → 1687). See claim-audit
M3. Lowest priority item in this entire list.

---

## What this list deliberately does not include

No edit here proposes adding a new test, a new feature, a new metric, or
new evidence of any kind. Every P0/P1/P2 item is either a correction to
existing text, a reconciliation between existing documents, or a
voluntary disclosure of a limitation already implied by code that exists
today (P1-3, found by reading `internal/decision/server.go`'s actual
restart behavior, not by building anything new). This is consistent with
the review task's own constraint: identify how to discuss limitations
intelligently, not how to remove them by implementing fixes.
