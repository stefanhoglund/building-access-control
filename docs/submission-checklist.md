# Submission Checklist

Status as of this review pass. Each item is marked **DONE**, **TODO**
(the student must act — this review cannot do it), or **N/A** with the
reason. This checklist does not itself change anything — it is the
to-do list the rest of this review pass produced.

## Report content

- [ ] **TODO — Dashboard screenshot.** `report/final-report.tex` §12
  still has the placeholder box (not real content): capture
  `http://localhost:8085/` after running a few scenarios (e.g.
  `make demo-after-hours` natively, or the Docker equivalent), save to
  `report/figures/dashboard.png`, replace the `\fbox{...}` placeholder
  block with `\diagramfig{dashboard.png}{0.9}{Dashboard...}`, rebuild.
  This is the **one** remaining placeholder in the document — confirmed
  by inspection, not assumed (see "no placeholder text" below).
- [ ] **TODO — Apply the P0 edits.** `docs/final-report-edit-list.md`
  lists 5 P0 items (REQ-AUTH table arithmetic, stale recovery-time
  number in 3 other documents, the unverifiable proposal-comparison
  claim, the nine-vs-ten-containers reconciliation, the self-graded
  Dimension-B sentence). None of these were applied during this review
  pass (frozen scope) — they need a short, separate edit pass before
  submission.
- [x] **DONE — Compile the final PDF.** `report/final-report.pdf` exists,
  21 pages, builds cleanly (`cd report && make`), no undefined
  references, no LaTeX errors. **Re-run `make` in `report/` after
  applying the P0 edits above** — the currently-built PDF does not yet
  reflect them.
- [x] **DONE (with the one TODO above) — No placeholder text.** Searched
  the report source for template leftovers (`\guidance`, `\fillin`,
  bracketed `[...]` slots) — none remain except the dashboard screenshot
  box, which is the single known, intentional placeholder.
- [ ] **TODO — Put the real repository URL in the report.** Corrected
  after checking directly (an earlier draft of this checklist wrongly
  assumed no remote existed): `git remote -v` shows a real GitHub remote,
  `git@github.com-personal:stefanhoglund/building-access-control.git`
  (HTTPS form: `https://github.com/stefanhoglund/building-access-control`),
  and `main` is tracking `origin/main` at the same commit as local HEAD
  (`59dff71`) — the repository *has* been pushed, up through that
  commit. `report/final-report.tex` §1's summary table currently just
  says `applied-ai-edge-computing (local Git repository; see
  README.md)` with no actual URL. Replace it with the real HTTPS URL
  above. **Also**: the 58 changed/untracked paths from this entire
  finalization/report pass are not yet pushed (see the git-status item
  below) — push after committing, before citing the URL as containing
  the submitted state.
- [x] **DONE — Diagrams legible.** All six C4/behavioral diagrams
  (context, container, component, dynamic, state, deployment) were
  re-rendered via Chrome headless (not the lossy cairosvg path that was
  tried and rejected first) and spot-checked visually at full
  resolution — text, arrows, and labels are all crisp. The six
  evaluation figures (latency, stress, recovery-time) were generated
  from real evidence files and are legible with titled axes and legends.
- [x] **DONE — Requirement/test references resolve.** Every `REQ-*` ID,
  test function name, and evidence-directory path spot-checked during
  this review (a representative sample, not exhaustively all ~100+
  citations) resolved to a real, existing thing in the codebase — no
  fabricated test names or non-existent paths were found. See
  `docs/final-report-claim-audit.md`'s "what was checked and found to
  hold up" section for the specific items verified.

## Contribution / authorship

- [ ] **N/A or TODO, confirm which** — Contribution statement. The
  rubric requires one "for pair projects." This repository's `git log`
  and the report's author line (`Stefan Hoglund`) are consistent with a
  solo project. **Confirm this is in fact a solo submission**; if it is
  not, a contribution statement (architecture/implementation/testing/
  evaluation/report-preparation breakdown per student) must be added to
  the report — it is currently absent.

## Evidence and reproducibility

- [x] **DONE — Evidence directories present and non-empty.**
  `evidence/manual-allow-passage/`, `evidence/manual-deny-passage/`,
  `evidence/decision-restart-fault/`, `evidence/manual-decision-restart-
  fault/`, `evidence/actuator-failure/`, `evidence/latency/`,
  `evidence/anomaly/`, `evidence/stress/`, `evidence/scenarios/`, and
  `evidence/final-verification/` all exist with real content (not
  stubs) — confirmed during the original implementation passes and
  re-confirmed by this review reading their contents directly.
- [x] **DONE — Compose instructions verified to work as documented.**
  `docker compose config/build/up` all succeed as described in
  `docs/deployment.md` and `README.md`; the door-seed retry fix is in
  place and reverified. (This review did not re-run the Docker stack —
  implementation is frozen — but confirmed the verification evidence for
  this already exists and is dated from the actual final-verification
  pass, not an earlier, possibly-stale run.)
- [ ] **TODO — Final git status / commit.** As of this review, the
  working tree has 58 changed/untracked paths relative to the last
  commit (`59dff71`, "Complete manual successful-passage CPS
  validation") — **nothing from the entire finalization, scenario/
  Docker, or report-writing work has been committed.** Before
  submission: review `git status`/`git diff` for anything unintended
  (e.g. confirm `bin/` and `.run/` — local build/runtime artifacts —
  are not accidentally staged), then commit **and push** — `origin/main`
  is currently also still at `59dff71`, so none of this work is on
  GitHub yet either. Decide whether to commit in one pass or in logical
  groups; either is defensible, but **do not submit before both
  committing and pushing** if the course expects the repository URL in
  the report to reflect the graded state.
- [ ] **TODO — Verify the PDF opens correctly** outside this
  environment. It was validated here with `pdflatex`'s own clean exit
  and a page-by-page visual render (via `pymupdf`) in this session — but
  it has not been opened in an actual PDF reader (Preview, Adobe
  Acrobat, a browser) on the machine that will be used to submit it.
  Do this once before submission as a final sanity check — cheap and
  catches font-embedding or renderer-specific issues this review's
  tooling can't.

## External / course-platform items (outside this repository's visibility)

- [ ] **TODO — Verify required Canvas deliverables.** This review has no
  visibility into the course's Canvas submission requirements (how many
  files, which formats, deadline, whether the repository itself must be
  submitted as a link/archive alongside the PDF, whether a video/demo
  recording is required). **Check the assignment page directly** — this
  checklist only covers what exists inside the repository.
- [ ] **TODO — Verify oral-defense readiness personally**, not just that
  the document exists. `docs/oral-defense-prep.md` (including the new
  Adversarial Examiner Questions section from this review) is thorough,
  but the rubric is explicit that the oral exam is an *individual*,
  live validation — reading the document is necessary but not
  sufficient. Before the exam: actually draw the architecture from
  memory without looking (§1 of that document), and actually attempt a
  few of the Adversarial Examiner Questions out loud, timed, without
  reading the prepared answer first, to find out which ones are
  genuinely internalized versus only recognized when read.

## Summary

Of the items above: **7 DONE**, **8 TODO**, **1 TODO-or-N/A pending
confirmation**. None of the TODO items require implementation work —
they are a report-edit pass (P0 items), a commit, a screenshot, and a
small number of external/personal verification steps outside this
repository's scope.
