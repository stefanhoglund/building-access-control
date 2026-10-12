# Oral Defense Preparation

Not a script. The oral exam validates that you personally understand the
system, not that you can recite this document — use it to check yourself,
not to memorize lines. If an answer here surprises you, go read the
actual code/doc it cites before the exam, don't just trust the summary.

## 1. Draw the architecture from memory

Nine boxes, drawn in this order, is enough:

1. **Occupant & Movement Simulator** (`:8081`) — owns model time, owns
   occupancy/door-position truth. Talks to BuildSim.
2. **Card Reader** — polls the Simulator, posts to Event/Data. No
   listener of its own.
3. **Event/Data Service** (`:8082`) — validates, stores evidence,
   forwards to Decision. The *only* writer of `data/evidence.jsonl`.
4. **Decision Service** (`:8083`) — authorization + anomaly + policy +
   runtime assurance, four modules, one process.
5. **Door Actuator** (`:8084`) — issues LOCK/UNLOCK to BuildSim, verifies,
   reports CONFIRMED/UNVERIFIED.
6. **DoorStateSensor** — polls BuildSim's door state directly (not the
   Simulator). Only producer of `DoorStateObservation`.
7. **OccupancySensor** — polls the *Simulator's* occupancy truth, never
   BuildSim's mirror.
8. **Dashboard** (`:8085`) — reads the evidence file directly, no API.
9. **BuildSim** (`:9090`, external, course-provided) — authoritative
   physical state: door lock/position, topology. Not the authorization
   engine, not the anomaly detector, not the occupant simulator, not the
   historical record.

Arrows to get right without looking: Reader→Simulator (poll),
Reader→EventData (POST), EventData→Decision (POST, forwarded),
Decision→Actuator (POST, only on ALLOW), Actuator→BuildSim (REST),
Actuator→EventData (outcome), Simulator→BuildSim (door position +
occupancy mirror, one-way), DoorStateSensor→BuildSim (GET),
DoorStateSensor→EventData (POST), EventData→Decision (forward of door-
state observations, to disarm/trigger relock), OccupancySensor→Simulator
(GET), OccupancySensor→EventData (POST), Dashboard→evidence file
(read-only, direct).

## 2. Responsibility of every component (one line each)

- **Simulator**: model time + occupant intent + occupancy/door-position
  truth. Causes physical transitions, never reports observing them.
- **Card Reader**: the only process that turns a Simulator presentation
  into an `AccessAttemptObservation`.
- **Event/Data Service**: validation boundary + sole evidence writer +
  the thing on the control path between sensor and decision (ADR-005).
- **Decision Service**: the only autonomous decision-maker. Internally:
  `authz.go` (deterministic precedence), `anomaly.go` (two-factor
  scorer), policy (`final_action` from authorization *only*), runtime
  assurance (relock timers).
- **Door Actuator**: the only process that writes `lock_state` to
  BuildSim. Verifies before claiming CONFIRMED.
- **DoorStateSensor**: the only producer of `DoorStateObservation` —
  polls BuildSim directly, not the Simulator.
- **OccupancySensor**: polls Simulator truth, never BuildSim's mirror —
  this is deliberate (ADR-006), so a sensor can be wrong independently of
  truth, which matters for fault experiments.
- **Dashboard**: pure read-only view of the evidence file, no state of
  its own.
- **BuildSim**: authoritative physical state, zero persistence across
  restarts, not any of the four "intelligence" roles above.

## 3. Complete ALLOW trace (say this out loud, time yourself)

1. Simulator enqueues a presentation (`card-alice`) for `reader-1`.
2. Card Reader polls, sees it, POSTs an `AccessAttemptObservation` to
   Event/Data.
3. Event/Data stamps `receive_time`, appends evidence, POSTs to
   Decision's `/decide`.
4. Decision runs `authorize()` → `ALLOW/ROLE_ALLOW`; runs the anomaly
   scorer independently → say `NORMAL`; policy sets `final_action=ALLOW`
   from authorization alone.
5. Decision POSTs the `AccessDecision` to Event/Data (evidence), then
   POSTs an `UNLOCK` command to the Actuator.
6. Actuator calls BuildSim's `SetDoorState` (GET-modify-PUT under the
   hood), verifies via `GetDoorState`, POSTs `CONFIRMED` outcome to
   Event/Data.
7. Simulator, watching for `UNLOCKED`, sets `door_position=OPEN` in
   BuildSim, sleeps past the sensor poll interval, updates its own
   occupancy truth (source −1, destination +1), mirrors both counts to
   BuildSim, then sets `door_position=CLOSED`.
8. DoorStateSensor (polling BuildSim independently) observes OPEN, then
   CLOSED, each POSTed to Event/Data and forwarded to Decision.
9. The OPEN observation disarms the no-open timer; the CLOSED observation
   (door was opened) triggers an immediate relock: Decision issues
   `LOCK`/`RELOCK_AFTER_PASSAGE`, Actuator confirms.
10. OccupancySensor, polling the Simulator, observes the new count and
    records it.

## 4. Complete DENY trace

Same steps 1–4, except `authorize()` returns `DENY/NO_PERMISSION` (or
whatever the actual reason is). Policy sets `final_action=DENY`. **No
command is ever issued** — `issueUnlock` is only called when
`final_action==ALLOW`. The door's logical state never leaves LOCKED.
Nothing physical happens; the Simulator's crossing watcher (which started
regardless, listening for `UNLOCKED`) times out after 5 wall-clock
seconds and gives up — this bounded wait, not a special "DENY" code path,
is what makes "an occupant never crosses a locked door" true structurally.

## 5. Anomaly vs. authorization — the one invariant to never get wrong

Authorization = "is this permitted" (deterministic precedence,
`authz.go`). Anomaly = "is this usual" (two-factor scorer, `anomaly.go`).
They are computed from different inputs (anomaly never sees the
authorization result), by different functions, and combined in exactly
one place: `final_action` is set *solely* from `auth.result`. Anomaly
only ever sets a separate `alert` boolean. Proven by a unit test that
asserts an anomalous-looking unauthorized attempt still DENYs
(`TestDecide_Unauthorized_DeniedRegardlessOfAnomaly`) — this is not a
convention, it's structurally true because the function that computes
`final_action` never reads the anomaly variable at all.

## 6. Commanded state vs. observed physical state

A `LOCK`/`UNLOCK` command being *sent* is not the same as the door
*actually* being locked/unlocked. The Actuator always verifies
(`GetDoorState` after `SetDoorState`) before claiming `CONFIRMED`; if
verification fails or BuildSim is unreachable, the outcome is
`UNVERIFIED` with an empty `observed_state` — never a false `CONFIRMED`.
This is why the state diagram (report §8.2) has a separate `UNVERIFIED`
node reachable from either LOCKED or UNLOCKED: it's a confidence
qualifier on the commanded state, not a third physical state.

## 7. Model time vs. wall-clock time

Model time: owned solely by the Simulator, used for permission windows,
scenario timing, the anomaly scorer's time-of-day factor, and the relock
timeout's 10-second window. Every other process reads it via
`GET /modeltime` or receives it stamped on an observation — nobody
infers it locally. Wall-clock time: used for network timeouts
(`internal/httpx`, 2s default), process health, and every latency/
recovery-time number in the evaluation. The two are never mixed: e.g.
the relock ticker *polls* on a wall-clock cadence but *compares* against
model time.

## 8. Why Event/Data Service is on the control path

Could have been Sensor→Decision directly, with Event/Data only getting a
copy for storage. Rejected (ADR-005) because that creates two sources of
truth for "was this observation accepted," and splits validation logic
across two places (or forces it to be skipped in one). Routing through
Event/Data gives one validation/evidence boundary, at the cost of putting
it on the latency-critical path — a real trade-off, not a free lunch;
decision latency was measured *with* this hop included (p95 = 0.601ms,
nowhere near the 10ms threshold, so the cost was judged acceptable, not
ignored).

## 9. Why authorization/anomaly/policy/runtime-assurance are separated logically but not as separate services

ADR-002. They're four internal modules of one process, not four network
services. Reasoning: the course requires *an* independently deployable
autonomous component, not one per internal function; splitting further
would add three more network hops/timeout surfaces/deployment units for
no required benefit. The separation that actually matters (authorization
can't see anomaly's conclusion, anomaly can't see authorization's) is
enforced in code — `authorize()` and `anomalyTracker.score()` never call
each other — which is a real boundary even without a network hop between
them.

## 10. Why one autonomous Decision Service (not per-door, not per-function)

With exactly one controlled door, splitting by door has no referent yet;
splitting by function was addressed above (ADR-002). One process keeps
the decision policy's central invariant (`final_action` from
authorization only) a single, auditable function instead of a contract
that would need to hold *across* a network boundary between two separate
processes — harder to guarantee, harder to test.

## 11. Why Docker Compose, not Kubernetes

Nine containers, mostly plain REST, one host. Course explicitly
discourages orchestration complexity "unless explicitly required by an
experiment" — none of this project's experiments need multi-node
scheduling, rolling updates, or a control plane. Compose's `depends_on` +
one shared image + two bind mounts is exactly the orchestration nine
independently-restartable-but-already-resilient processes need.

## 12. Why REST, not MQTT/a message broker

Every inter-process call here is naturally request/response in shape
("decide this," "confirm this command") — no pub/sub fan-out, no need to
buffer messages for an offline consumer. A broker adds a piece of
infrastructure, a deployment unit, and a failure mode (the broker itself)
without an offsetting benefit for this shape of traffic. REST with a
bounded 2s timeout on every call is precise enough to document per-
endpoint and simple enough that every fault experiment in this project
could inject a failure at an exact, nameable point.

## 13. Why simple rule-based anomaly scoring, not ML

Two reasons, in order of how defensible they are under questioning: (1)
with one controlled door and eight simulated people, there is no
realistic quantity or diversity of data to train or validate a model
against — building one now would be theater, not engineering. (2) A
rule-based baseline makes the system's one non-negotiable safety
invariant (anomaly can never grant access) a structural, one-line-
provable guarantee instead of a property you'd have to re-verify after
every retrain. The course's own grading philosophy states explicitly
that more sophisticated AI does not by itself earn a higher grade.

## 14. What fails if the Decision Service dies

No new decisions get made. Any pending presentation's observation still
gets recorded by Event/Data (POST never blocks on Decision being up —
Event/Data logs the forward failure and continues), but no `AccessDecision`
is produced, so no command is ever issued. A door that was locked stays
locked (REQ-RES-001). On restart: a fresh process with no carried-over
state immediately resumes correct decisions — measured at 10.83–32.26ms
from restart to first correct ALLOW/UNLOCK across 10 runs. Other eight
processes are completely unaffected the whole time.

## 15. What fails if the Actuator dies

Decision still produces correct decisions and still *issues* commands —
it has no visibility into whether the Actuator is reachable. The command
is recorded as evidence regardless (posted before delivery is attempted).
If delivery to the Actuator itself fails (not just Actuator→BuildSim),
**no outcome record is ever produced at all** — this is the
Decision→Actuator missing-outcome gap the stress experiment surfaced,
worse than `UNVERIFIED` because there's no record anything was even
attempted. If the Actuator is reachable but BuildSim isn't, the outcome
comes back `UNVERIFIED`, never a false `CONFIRMED`.

## 16. What fails if BuildSim dies

Three independent processes are affected simultaneously: the Actuator
(can't verify/confirm commands), the Simulator (can't write door position
or mirror occupancy — both log errors and keep retrying on the next
event), and DoorStateSensor (polls fail, no new observations). BuildSim
is a genuine single point of failure shared by three processes — named
explicitly as a limitation (`evidence/actuator-failure/README.md`), not
discovered and hidden. On restart, BuildSim has zero persistence (by
design, confirmed from its own source) — door-1 must be re-seeded
(natively: `scripts/run-local.sh seed`; in Compose: the `door-seed` init
container, now retry-hardened after a race was found during this pass).

## 17. What the stress test revealed

Concurrency swept 1→3000 directly at Event/Data's `/observations`.
Throughput collapses from 1794 req/s (concurrency 50) to **zero** between
50 and 100, staying at zero through 1500, partially recovering to 1687
req/s at 3000 with 73% errors. Root cause, confirmed not assumed: the
Decision Service's own log shows a loopback port-assignment failure on
its call to the Actuator — every inter-service call shares
`http.DefaultClient`, whose default transport allows only 2 idle
connections per host, and each access attempt fans out into ~4 internal
HTTP calls. Confirmed by first fixing a benchmark-tool artifact
(undrained response bodies) and re-running: the ceiling didn't move,
proving it's a production-code property, not a test artifact. Not fixed
— the point was to find and honestly report the limit, not move it. It
also surfaced the Decision→Actuator missing-outcome gap (see Q15).

## 18. What you would change for 100 doors / multiple buildings

Be ready to reason live here, not recite: the per-door control state in
Decision is already a map keyed by door ID, so it generalizes
structurally — but it's single-slot per door (see the overlap limitation
in report §13), which would need to become a queue or per-decision keying
under real multi-door contention. The single JSONL evidence file would
need partitioning or a real datastore — it was sized for this project's
evaluation volume, not for scale. BuildSim's wholesale-replace PUT
semantics (GET-modify-PUT today) would become an actual throughput
concern, not just an adapter-design curiosity, as the door/room
collections grow. The connection-pool ceiling found in the stress test
would need raising *before* 100 doors' worth of traffic, not after.

## 19. What requirements remain incomplete (say the themes, not just the IDs)

Two themes, both named consistently throughout the project, not
discovered late: **device re-registration** after a restart (no
`/api/equipment` flow — REQ-DIST-005/006), and **data-quality fault
handling** (no duplicate, late/out-of-order, or meaningfully-handled
malformed-observation detection anywhere in the pipeline — REQ-AUTH-005,
REQ-DATA-004/005/006, REQ-RES-003/005). This second theme is also
Dimension C's grade-5 capping weakness (`docs/report-rubric-audit.md`) —
know that connection if asked.

## 20. Strongest and weakest architectural decisions

**Strongest**: separating authorization and anomaly as independently
computed signals combined in exactly one place (Q5/Q9). It's the one
decision that is simultaneously a safety property, a unit-testable
invariant, and the thing that makes the whole "autonomous but not
unaccountable" story in the report coherent.

**Weakest**: the Decision Service's per-door control state being keyed
only by `door_id`, not `decision_id` — a single overlapping presentation
can clobber an earlier unlock's relock bookkeeping (found during Docker
verification, report §13). It's a real, if narrow, gap in an otherwise
carefully-reasoned component, and it's the kind of thing that would need
fixing before this design could honestly claim to handle more than one
presentation in flight per door.

## 21. Three most important trade-offs (have these ready verbatim-ish)

1. **Event/Data on the control path** (Q8): one validation/evidence
   boundary, at the cost of latency-path length — judged acceptable only
   because it was *measured*, not assumed.
2. **Rule-based anomaly, not ML** (Q13): structural safety guarantee and
   honest data-scarcity reasoning, at the cost of an anomaly detector
   that cannot adapt beyond two fixed factors.
3. **One Decision Service process, four internal modules** (Q9/Q10):
   keeps the core safety invariant a single auditable function, at the
   cost of all four modules sharing fate on every restart (though restart
   is fast and measured, Q14).

## Likely examiner questions, grade 4–5 register (≈20, with concise answer notes)

1. *"Walk me through what happens, in order, if two people present
   credentials at the same door within a second of each other."* — Both
   get queued FIFO on the Simulator's per-reader pending list; Card
   Reader drains one at a time; each gets its own observation/decision
   normally, but if both ALLOW and their crossings overlap, the known
   per-door-state limitation (Q20) can apply to the relock bookkeeping —
   say this honestly, don't pretend it's handled.
2. *"Why does the Event/Data Service write evidence before the Actuator
   confirms anything?"* — Because the command being *issued* is itself an
   evidential fact (REQ-DEC-004/007's traceability requirement), separate
   from whether it was later confirmed — this is also why the
   Decision→Actuator missing-outcome gap (Q15/Q17) is possible, and why
   that's reported as a real gap, not swept under "it's fine, we log
   everything."
3. *"What would you have measured that you didn't?"* — Actuation-
   confirmation latency against the *real* BuildSim adapter, not
   `FakeBuildSimClient` — stated explicitly as a caveat (report §11.2).
4. *"If I set the anomaly threshold to 0.3 instead of 0.5, what
   actually changes, concretely?"* — A single unusual-time OR single
   repeated-attempt (each weight 0.6) would already exceed 0.3 alone
   (currently both already do at 0.5 too, since each factor is 0.6) — so
   at today's weights 0.3 vs 0.5 doesn't change classification at all;
   it would only matter if a future third, smaller-weighted factor were
   added. Know this precisely, don't guess live.
5. *"Why 07:00–19:00 for the time-of-day factor and not something
   data-derived?"* — No evaluation dataset exists for this project;
   stated as a "defensible starting baseline," not derived — say that
   plainly if asked, don't invent a justification on the spot.
6. *"What happens to in-flight requests when you `docker compose
   restart decision`?"* — They either complete against the old process
   (if already past the point of calling Decision) or fail and are
   logged by Event/Data without crashing; the next observation after
   restart is handled by the fresh process. No request is silently lost
   from evidence — it's either recorded fully or recorded as an
   observation with nothing following, same as Q15's gap.
7. *"Is BuildSim ever the bottleneck in your stress test?"* — No —
   confirmed the bottleneck is the Decision→Actuator loopback connection
   pool, before the request would even reach BuildSim; name the exact log
   line if asked (`dial tcp ...: can't assign requested address`).
8. *"Could the door-seed race have caused a false unlock?"* — No —
   walk through why: the failure mode was door-1 *not existing* in
   BuildSim at all, which the Actuator/Simulator calls would simply fail
   against (connection/404-shaped errors), not silently succeed as if
   locked/unlocked. Be ready to explain this precisely, it's a natural
   follow-up to "you found a bug, did it compromise safety?"
9. *"Why don't sensors read from each other instead of polling BuildSim/
   the Simulator independently?"* — Because independent polling is what
   lets a sensor be *wrong* independently of truth, which is required for
   fault/anomaly experiments (ADR-006) — a shared read path would remove
   that independence.
10. *"What's the actual cost, in requests, of one access attempt?"* — ~4
    internal HTTP calls fan-out from one observation (this is the number
    behind the stress-test root cause) — know this number.
11. *"If you had one more week, what's the single highest-value thing to
    build?"* — Data-quality fault handling in Event/Data (duplicate/late/
    malformed) — cheap relative to full behavioral simulation, no new
    process, and it's explicitly the Dimension C grade-5 gap.
12. *"Why didn't you fix the connection pool bottleneck — it's a
    one-line change?"* — Deliberate: this pass's purpose was to find and
    honestly report the system's limit, not move it; say this plainly,
    it's a stated methodological choice, not an oversight.
13. *"What's the difference between `UNVERIFIED` and the
    Decision→Actuator missing-outcome gap?"* — `UNVERIFIED` is a record
    that something was attempted and failed to confirm; the missing-
    outcome gap is a command recorded with *no* resolution record at
    all — strictly worse, because there's nothing to show it was even
    tried.
14. *"How do you know the relock timeout is actually measured in model
    time, not wall-clock?"* — The ticker *polls* on a wall-clock cadence
    but *compares* the elapsed duration against model time read from the
    Simulator — point to the exact comment in the code if pressed.
15. *"What does 'independently deployable' actually buy you, concretely,
    beyond satisfying the course checklist?"* — Directly demonstrated:
    killing Decision or Actuator never required restarting any of the
    other eight processes, confirmed by uptime in both the native and
    Docker fault experiments.
16. *"Your population has 8 people — why not more, and does that limit
    anything you claim?"* — Deliberate: "behavioral diversity, not
    population size" — the 8 people span every authorization role the
    config defines; more people would not exercise any code path this
    design doesn't already. It does limit REQ-SIM-001/002 to PARTIAL,
    honestly, since a larger, more varied population is part of what full
    behavioral-profile generation would need.
17. *"What's one thing you were wrong about during this project?"* —
    Good answer shape: name the door-seed race (assumed `docker compose
    restart` behaved like `up` with respect to `depends_on`; it doesn't)
    or the per-door overlap limitation (assumed one-in-flight-per-door
    was enough; a self-inflicted overlapping test sequence proved it
    isn't under concurrent presentations) — both are real, both were
    found by your own verification, not an examiner.
18. *"Explain why the anomaly score for a repeated attempt and an
    unusual-time attempt happening together isn't additive beyond what
    you'd expect."* — It *is* additive (both factors sum): a card that's
    both late-night AND repeated scores 1.2, still just classified
    ANOMALOUS (same as either alone, since the cutoff is 0.5) — the score
    is retained and recorded even though the classification saturates;
    know this is a deliberate simplification, not a hidden bug.
19. *"If BuildSim's API changed tomorrow, how much of your system would
    break?"* — Only `internal/buildsim/http_client.go` — the adapter is
    the one deliberate seam; everything else talks to the `Client`
    interface, and the entire automated test suite runs against
    `FakeBuildSimClient` for exactly this reason.
20. *"Defend using a plain JSONL file as your 'historical evidence
    store' with a straight face."* — One writer (ADR-005), no concurrent-
    write hazard to design around; trivially restart-survivable,
    directly demonstrated; every query this project ever needed was a
    linear scan over a file that stayed in the hundreds-of-KB range for
    its whole evaluation campaign — a database would have added an
    operational dependency for a capability never actually needed.

## Adversarial Examiner Questions

Written by playing the skeptical examiner against this project's own
evidence, not against a sympathetic reading of it. Each item: the
question, the strongest answer that uses *only* evidence that actually
exists (no invented numbers, no claims stronger than what was measured),
what not to say even though it would sound better, and the follow-up
that's likely to come next. If an answer here leans on "we don't know" or
"not tested," say exactly that in the exam — it is a better answer than
a confident guess.

### A. Architecture

**A1. "Why this decomposition — eight processes, specifically? Why not
five, or fifteen?"**
Answer: The decomposition follows the course's own mandatory boundary
(sensor/actuator/decision must be independent, restartable processes)
plus the natural data-flow seams the design already has: three sensor
types (card, occupancy, door-state) are physically distinct observation
sources so each gets its own process; actuation and decision are
separated because the course requires the decision-maker to be
independently deployable from what it commands; Event/Data is separate
because it's the evidence/validation boundary (ADR-005); the dashboard is
separate because it's a pure reader with no business logic. Eight is the
count that falls out of "one process per distinct responsibility," not a
target chosen in advance.
What not to claim: that eight is "optimal" in any measured sense — no
alternative decomposition was built and compared.
Follow-up: "Which two of these eight could you merge with the least
loss, and what would you lose?" — be ready to answer this live (a
defensible answer: Card Reader and OccupancySensor are the most similar
in shape — both pure pollers with no listener — merging them would lose
independent restartability of one without the other, which is exactly
the mandatory criterion being tested).

**A2. "Why does Event/Data sit on the control path instead of being a
pure logging sink?"**
Answer: ADR-005's explicit trade-off: routing Sensor→Event/Data→Decision
gives one validation/evidence boundary instead of two potential sources
of truth about "was this observation accepted." The measured cost is one
extra network hop on the decision-latency path — and that cost was
measured, not ignored: p95 decision latency with this hop included is
0.601 ms against a 10 ms threshold (`evidence/latency/results.json`).
What not to claim: that this was "free" — it isn't; it's a real
trade-off that happened to measure out cheap in this deployment, at this
load. The stress experiment (§11.5 of the report) shows it stops being
cheap well before the system's other limits are reached.
Follow-up: "At what load does that hop become the bottleneck instead of
the connection pool?" — honest answer: not measured; the stress
experiment found the connection pool breaks first (concurrency ~100),
so the control-path hop's own cost under load was never isolated.

**A3. "Why REST instead of a message broker — isn't that just avoiding
more infrastructure, not an architectural reason?"**
Answer: It is partly exactly that, stated plainly (the course explicitly
discourages infrastructure without a required benefit) — but there's a
positive reason too: this system's traffic is uniformly request/response
in shape ("decide this," "confirm this command"), with no fan-out,
no offline buffering requirement, and no case anywhere in the design for
one message having multiple independent consumers. A broker's core value
proposition (decoupled pub/sub, delivery guarantees across disconnected
consumers) has no referent in this system's actual traffic pattern.
What not to claim: that REST is "more resilient" than a broker would have
been — that was never tested; the fail-closed/bounded-timeout properties
demonstrated here (§11.3) are a property of `internal/httpx`'s design
choices, not of REST-the-protocol specifically — an MQTT-based system
could have the same properties with equivalent engineering.
Follow-up: "What would force you to add a broker later?" — a defensible
answer: multiple independent consumers needing the same event (e.g., a
second dashboard, an external alerting system) or a need to buffer
events while a consumer is offline for longer than a retry budget covers
— neither exists in this system today.

**A4. "Why one Decision Service process instead of one per door, given
you admit the per-door state is already a map?"**
Answer: With exactly one controlled door, splitting by door has no
referent to split *against* — there's only one entry in the map. The
per-door map was written generically (keyed by door ID) because it costs
nothing to do so, not because multi-door operation was built or tested.
What not to claim: that the current single-process design would scale
cleanly to many doors under real concurrent load — report §13 names the
specific, honest reason it might not (the per-door state is single-slot,
and a self-inflicted overlapping-presentation test during Docker
verification already found it can be clobbered with just *one* door
under artificial overlap).
Follow-up: "You found that bug with one door. What does that predict
about ten doors under real concurrent traffic?" — honest answer: nothing
precise — the bug was found by deliberately racing two presentations on
the same door; with many doors the per-door keying means different doors
don't interfere with each other, so the *same* door would need
*repeated* overlap to reproduce it, which is plausible but was never
tested at any door count other than one.

**A5. "BuildSim is a single point of failure for three of your
processes. Isn't that a basic architecture mistake?"**
Answer: It's a real, acknowledged trade-off, not an oversight — BuildSim
is the course-provided external system representing the physical
building; nothing in this project owns or could own an alternative
representation of "the building" without inventing a second BuildSim.
The honest framing (used in the report, §13) is that this is a
consequence of the *problem statement* (BuildSim is given, external, and
authoritative for physical state per CLAUDE.md §4), not a design choice
that could have been made differently within this project's scope.
What not to claim: that this is "fine" or "doesn't matter" — it's named
explicitly as a found limitation (`evidence/actuator-failure/README.md`)
precisely because it does matter: three processes degrade simultaneously
when BuildSim is down, and only one of those three (the Actuator) was
ever directly asserted on in a test.
Follow-up: "What would mitigate this without replacing BuildSim?" —
defensible answer: nothing was built, but the honest shape of a fix is a
local cache of last-known BuildSim state per dependent process, so
degradation is graceful (stale-but-available) rather than immediate —
named here as unbuilt, not claimed as done.

**A6. "Where exactly does this architecture stop scaling, concretely —
not in general terms?"**
Answer, concretely, from report §13: the single JSONL evidence file (no
partitioning, no index — a linear scan was adequate for a
hundreds-of-KB file across this project's whole evaluation campaign, not
necessarily at 100x the data); the single-slot per-door Decision state
(already found fragile under overlap with *one* door); BuildSim's
wholesale-replace PUT (every door-state write today re-sends the
*entire* door collection — fine at N=1 door, not fine at N=100).
What not to claim: a specific number of doors or requests/second at which
it breaks — none of these three limits were load-tested at any scale
other than this project's own (one door, population of eight, the
stress experiment's synthetic concurrency which exercises the *decision
pipeline*, not the evidence file or BuildSim's PUT payload size).
Follow-up: "Which of those three would you expect to break first?"
— reasoned (not measured) answer: BuildSim's wholesale-replace PUT,
because its cost grows with total door count on *every single write*,
not just under concurrent load — the other two degrade under volume, this
one degrades under topology size alone.

**A7. "Which of your architectural decisions were actually driven by
evidence, rather than just argued for?"**
Answer, honestly: one, clearly — REST-with-bounded-timeouts over a
broker is supported by the fact that the stress experiment could inject
and attribute a failure to an exact, named point (`internal/httpx`'s
connection pool) specifically *because* the interfaces were simple
enough to reason about precisely; that's indirect evidence the choice
paid off, not a controlled A/B comparison. The other five decisions in
report §4.4 (one-process Decision Service, Event/Data on the control
path, JSONL over a database, rule-based over ML anomaly, Compose over
Kubernetes) are each reasoned against a named alternative but none of
those alternatives was built far enough to produce a comparable number.
What not to claim: that all six decisions are "evidence-based" — the
report itself (after this review pass) should not claim more uniformity
here than actually exists.
Follow-up: "If you had to pick one alternative to actually build and
measure, which would most change your mind if it won?" — a fair, honest
answer: a microservice-per-Decision-module variant, measured for latency
overhead, since that's the one alternative where "it would obviously be
worse" is argued but never shown.

### B. Systems behavior

**B1. "Walk me through process failure, generically — what's the
invariant that holds no matter which process dies?"**
Answer: No process's death can cause a false positive physical action
(a door unlocking without a corresponding ALLOW decision) — every
process failure mode tested (Decision killed, Actuator killed, BuildSim
killed) degrades to "nothing happens" or "an honestly-marked
UNVERIFIED/absent record," never to "the wrong thing happens
confidently." This was verified for two of nine processes directly
(Decision, Actuator/BuildSim link); the other six (three sensors, Card
Reader, Simulator, Dashboard) were not independently fault-injected.
What not to claim: that this invariant was proven for all nine processes
— it was architecturally reasoned to generalize (same restart pattern,
same bounded-timeout calls) but only directly demonstrated for two.
Follow-up: "What's the one process whose failure you're least confident
about, and why?" — honest answer: the Simulator, because it owns model
time and occupancy truth; if it died mid-crossing, no automated test
exercises what state the system is left in (an occupant "in transit"
with no further truth update) — this specific scenario was never run.

**B2. "If the Decision Service dies right after issuing an UNLOCK but
before the door opens, is the door now unlocked with nobody watching
it?"**
Answer: The door is physically unlocked (BuildSim's state), but Decision
owned the no-open relock timer for it — if Decision is down, that timer
isn't running. On restart, Decision starts with **no carried-over
state** (confirmed, `decision_restart_test.go` and the real-process
capture) — meaning the restarted process does not know this door has an
outstanding unlock to watch. This is a genuine gap: nothing in the
current design re-derives "this door is unlocked and nobody's watching
it" from BuildSim's own state on Decision startup.
What not to claim: that this is handled — it is not, and this review
found it by asking the question, not by citing an existing test. It
should be named as a limitation, not glossed over.
Follow-up: "Is this the same as the per-door overlap bug you already
found?" — no, it's related but distinct: the overlap bug is about two
*concurrent* presentations on one door; this is about Decision Service
downtime coinciding with a *physically open commitment* with no
reconciliation step on restart. Say this distinction clearly if asked —
don't conflate them to make it sound like one known issue covers both.

**B3. "What happens to the other two processes that also depend on
BuildSim (Simulator, DoorStateSensor) when it disappears — you only
tested the Actuator path?"**
Answer, honestly: not directly asserted on by any test. The architecture
predicts graceful degradation (both make bounded-timeout calls through
the same `internal/httpx`, so neither hangs), and `evidence/actuator-
failure/README.md` names this exact gap explicitly ("killing it affects
all three simultaneously... unobserved by any assertion here"). This is
a reasoned prediction, not a verified fact.
What not to claim: that this was tested "because the architecture is
uniform" — uniform bounded-timeout *calling code* is not the same as a
verified *outcome* for the Simulator's occupancy-mirroring write path or
DoorStateSensor's poll loop specifically.
Follow-up: "What would you actually expect to see in the evidence log
during that window?" — reasoned answer: repeated logged errors from both
processes' BuildSim calls, and simply *no new* DoorStateObservation or
mirrored-occupancy records for the outage's duration — not a crash, not
a corrupted record, just silence — but say this is a prediction, not an
observed result.

**B4. "What happens if two scenarios overlap — you found a bug here.
Walk me through exactly how, and what it does and doesn't prove."**
Answer: During Docker verification, trailing presentations from a
`rapid-repeat-access` run overlapped a fresh `decision` container restart
and a new `authorized-passage` attempt on the *same* door. The Decision
Service's per-door pending state (keyed by `door_id`, not
`decision_id`) let the second unlock's bookkeeping overwrite the first's,
so the first unlock's relock obligation was lost — the door ended up
stuck UNLOCKED. A clean, isolated request immediately after completed
correctly, which is what shows the restart itself wasn't the cause — the
overlap was.
What not to claim: that this proves anything about *many-door*
contention — it was found and reproduced with exactly one door and a
self-inflicted test sequence, not discovered under organic load.
Follow-up: "Could an attacker exploit this deliberately — present twice,
quickly, to leave a door unlocked?" — honest, uncomfortable answer: based
on the mechanism described, plausibly yes, if they could get two ALLOW
presentations in flight on the same door close enough together — this
was not tested as a security scenario, only as a reliability one.
Naming this possibility unprompted, if asked a safety-adjacent question,
is better than being caught not having considered it.

**B5. "What happens with stale or contradictory sensor data? You claim a
quality field exists — where's the evidence it's acted on?"**
Answer, honestly: `ObservationQualityOK` exists as a field value in the
contracts (REQ-DATA-003's required fields), but there is no logic
anywhere that ever sets anything *other* than OK, and no staleness check
exists (REQ-RES-003, REQ-RES-005 are both UNIMPLEMENTED). A stale
observation today is treated identically to a fresh one — there is no
"contradictory data" handling at all.
What not to claim: that the quality field provides any actual protection
— it's a schema placeholder today, not an active data-quality
mechanism. This is this project's single most defensible "grade 5,
Dimension C" gap — say so plainly if asked, don't try to make the
placeholder sound like a feature.
Follow-up: "If you had to add one check first, which, and why?" —
duplicate `event_id` detection is the cheapest (single hash-set lookup
in Event/Data, no new process), and the course's own Dimension D examples
name "injecting corrupted or missing data" explicitly — a strong,
honest, prepared answer.

**B6. "What failure can actually cascade in this system — not
hypothetically, from evidence?"**
Answer: BuildSim going down is the one demonstrated multi-process
cascade (Actuator, Simulator, DoorStateSensor all degrade at once,
§B3/B5 above) — though only one of the three was directly asserted on.
The stress experiment is the second: connection-pool exhaustion in one
shared client (`http.DefaultClient`) affects *every* process's outbound
calls simultaneously, including calls to BuildSim itself (confirmed by
reading `internal/buildsim/http_client.go` — it uses the same
`internal/httpx` package) — a single shared resource whose exhaustion is
genuinely systemic, not local to one link.
What not to claim: a cascade through the *evidence pipeline* itself
(Event/Data failing causing Decision to cascade-fail, etc.) — never
observed; Event/Data's forward-to-Decision failures are logged and
swallowed, not propagated.
Follow-up: "Is the connection-pool issue a 'cascading failure' in the
rubric's sense, or just a shared resource limit?" — fair question,
answer honestly: arguably the latter more precisely — it's a resource
contention effect (many processes competing for one global pool), which
the rubric also explicitly names as a grade-5 Dimension B example
("resource contention... communication bottlenecks") — don't oversell it
as "cascading failure" if pressed on the distinction, the more precise
term is also in the rubric and is a correct fit.

**B7. "What does the stress result actually *prove*, precisely — not what
does it suggest?"**
Answer: It proves that under this specific benchmark's load pattern
(concurrent senders POSTing directly to `/observations`, bypassing the
Card Reader), throughput collapses to zero between concurrency 50 and
100, and that the collapse is caused by `http.DefaultClient`'s
2-idle-connection-per-host default being exhausted by this system's
~4x-internal-fan-out per request — confirmed by the Decision Service's
own log line, and confirmed *not* to be a benchmark-tool artifact by
fixing one plausible artifact (undrained response bodies) and observing
the ceiling didn't move.
What not to claim: that this is "the system's" breaking point in any
deployment-representative sense — real credential presentations arrive
through the Card Reader's single poll loop, which the benchmark
deliberately bypasses specifically because it's a *different*,
uninteresting bottleneck; the *realistic* arrival-rate breaking point
(through the actual Card Reader path) was never measured and is almost
certainly far higher than 100 concurrent, because the Card Reader
itself serializes presentations.
Follow-up: "So could this ever actually happen in the deployed system as
built?" — honest answer: not through the Card Reader's normal operation
as built today, but it absolutely could if any future entry point (a
second Card Reader instance, a bulk-import endpoint, a different
scenario kind) ever posted to `/observations` concurrently at this
volume — naming this conditional clearly is better than either
dismissing the finding or overclaiming its real-world likelihood.

### C. Autonomous / data-driven behavior

**C1. "Is your anomaly detector actually 'data-driven,' or is it just
another rule system wearing an AI label?"**
Answer, honestly: it is a rule system — two fixed, hand-chosen weighted
factors and a fixed threshold, not fit or learned from any data. The
report does not claim otherwise (§4.4 explicitly justifies *not* using a
learned model). "Data-driven" in this project means the decision
consumes real-time sensor data (card ID, model time) through a defined
pipeline, not that the rules themselves were derived from data.
What not to claim: that this is "data-driven" in the machine-learning
sense, or that the two factors/weights have any empirical grounding —
report §13 already states the thresholds are "a defensible starting
point, not derived from a real evaluation dataset" — repeat that framing
exactly if asked, don't improvise a justification for the specific
numbers.
Follow-up: "What would it take to make it genuinely data-driven?" — a
labeled dataset of real access events distinguishing normal from
anomalous behavior, which doesn't exist for this project and wasn't
fabricated to pretend it does — a correct, honest answer.

**C2. "Why is authorization deterministic but anomaly detection isn't
(in the sense that it outputs a continuous score)?"**
Answer: They answer different questions with different tolerance for
ambiguity. Authorization is a yes/no gate with safety consequences — it
must be the same answer every time for the same inputs (REQ-AUTH-008,
directly tested — `TestAuthorize_Deterministic`). Anomaly is a
*graded* signal by nature ("how unusual," not "is it allowed") — a
continuous score is the more honest representation of that question,
even in a simple rule system, because "exactly how anomalous" is
genuinely a matter of degree in a way "is this permitted" is not.
What not to claim: that the anomaly score's specific numeric value
(0.6, 1.2, etc.) carries any calibrated meaning beyond "which factors
fired" — it's a sum of fixed weights, not a probability.
Follow-up: "Could authorization have been probabilistic too — e.g.
'probably authorized'?" — no, and say why clearly: the mandatory safety
invariant (an invalid/disabled card must never unlock a door) requires a
hard boundary; a probabilistic authorization gate would need an explicit
accept/reject threshold anyway, which just reintroduces the same
deterministic boundary one level down.

**C3. "Prove to me, right now, that anomaly literally cannot override
authorization — not 'we designed it that way,' prove it."**
Answer: `handleDecide` computes `auth := authorize(...)` and
`anom := s.anomaly.score(...)` independently (anomaly is never passed
the authorization result — check the function signature:
`score(cardID string, modelTime time.Time) anomalyOutcome`, no auth
parameter), and `final_action` is assigned from `auth.result` alone in
an if/else that never reads `anom`. This is directly tested:
`TestDecide_Unauthorized_DeniedRegardlessOfAnomaly` constructs an
attempt that is both unauthorized *and* anomalous and asserts the result
is still DENY.
What not to claim: that this is merely "well-tested" — it's
*structurally* impossible given the function's data flow, which is a
stronger and more precise claim than "we have a test for it," and the
stronger claim is the one to make, correctly.
Follow-up: "What if a future developer refactors this and breaks the
separation by accident?" — honest answer: the unit test would catch it
(it would fail), but there's no *type-system*-level guarantee (e.g., no
separate types that structurally can't be combined) — it's enforced by
a function body and a test, which is good but not the strongest possible
guarantee; say this precisely rather than claiming it's impossible to
ever break.

**C4. "What actually happens with a missing or corrupted observation,
concretely — walk me through the code path?"**
Answer: A malformed JSON body to `POST /observations` is rejected with
HTTP 400 by Event/Data's JSON decode step — confirmed behavior, not
assumed. What happens next is the honest gap: the rejected input is
**not retained as evidence and not counted by any telemetry**
(REQ-DATA-006, PARTIAL in `docs/traceability-matrix.md`) — it simply
vanishes from the system's observable history. A *missing* observation
(one that's silently never sent) produces no error at all — there's
nothing to detect its absence (REQ-RES-005, UNIMPLEMENTED).
What not to claim: that malformed data is "handled" — rejection without
retention or counting is a weaker claim than "handled appropriately,"
and the rubric's Dimension C grade-5 language specifically asks for the
stronger version.
Follow-up: "Isn't 'reject with 400' basically handling it?" — a fair
push; the honest distinction to draw: 400 prevents *bad data from
entering the decision path* (a real, working safety property), but
doesn't provide *observability into how often or why* it's happening,
which is what "identified" in the rubric's wording is really asking for.

**C5. "What's the single biggest thing capping Dimension C below 5 — not
in general, specifically?"**
Answer: Data-quality handling is unimplemented, not partially built —
zero duplicate detection, zero late/out-of-order detection, and
malformed-data handling that rejects but doesn't retain or count. The
rubric's own grade-5 wording for Dimension C is specific: "missing,
delayed, contradictory, or corrupted data are identified **and handled
appropriately**." This project can show the autonomous decision
component is well-evaluated (anomaly scenarios, latency, REQ-DEC-002/003
proven structurally) — that supports grade 4 — but the *data engineering
half* of Dimension C's grade-5 bar has no corresponding evidence at all.
What not to claim: that the anomaly/authorization separation
*substitutes* for data-quality handling — they're different questions
(one is about decision-signal independence, the other about pipeline
input quality) and conflating them to argue for grade 5 would be exactly
the kind of self-serving reasoning a critical-thinking-graded exam
should avoid.
Follow-up: "If you'd built just one data-quality check, which would move
the needle most for this dimension?" — duplicate detection: cheapest,
most directly named in the rubric's own Dimension D examples, and would
produce a new, demonstrable piece of evidence using the exact capture
pattern already used four times elsewhere in this project.

**C6. "Is there a real interaction between your anomaly scoring and
your data pipeline, or are they just two unrelated project parts you're
describing together?"**
Answer: There is a real, if narrow, interaction: the anomaly scorer's
repeated-attempt factor depends on `lastAttempt` state keyed by card ID,
which is itself built from the sequence of observations the pipeline
delivers — if the pipeline ever delivered observations out of order or
duplicated, the anomaly scorer's repeated-attempt classification would
be directly corrupted by it (a duplicate observation would look like a
genuine rapid repeat; an out-of-order one could reset or skip the
window incorrectly). This is a genuine, nameable interaction between the
two halves of Dimension C — and it was never tested, because the
data-quality side that would corrupt it doesn't exist yet to inject.
What not to claim: that this interaction has been evaluated — it's a
correctly-reasoned prediction of *where* a data-quality gap would bite
the autonomous component specifically, not a demonstrated failure mode.
Follow-up: "So a duplicate card presentation would falsely trigger an
anomaly alert today?" — yes, precisely, and this is worth saying
unprompted if the conversation gets here: today's pipeline has no
duplicate detection, so a literal network-level retry of the same
presentation would be scored as a second, independent, genuinely
repeated attempt by the anomaly logic, indistinguishable from the real
thing.

### D. Evaluation

**D1. "Were your latency measurements realistic, or did you measure the
easiest possible case?"**
Answer, honestly: the easier case. `cmd/latencybench` drives 200
presentations in-process, over loopback, against `FakeBuildSimClient` —
no real network hop to BuildSim, no real BuildSim response-time
variance. The report states this caveat explicitly (§11.2) rather than
letting the reader assume otherwise.
What not to claim: that 0.601 ms p95 represents real deployed latency
against the real BuildSim adapter — it represents the *pipeline's own*
overhead with the slowest real external dependency replaced by an
instant in-memory fake.
Follow-up: "What would you expect the real number to be?" — honest
answer: not measured, so no specific number should be given — reasoned
bound: bounded above by `internal/httpx`'s 2-second timeout per call,
and BuildSim's own real response time (not benchmarked in this project)
would need to be added on top of the measured 0.601 ms baseline.

**D2. "Which of your measurements used the fake BuildSim client and
which used the real one — list them precisely."**
Answer: **Fake** (`FakeBuildSimClient`): the latency benchmark (§11.2,
both decision and actuation-confirmation latency), the stress benchmark
(§11.5), and most of the unit/e2e test suite. **Real BuildSim binary**:
`test/integration/realbuildsim_test.go`, the manual ALLOW/DENY captures
(`evidence/manual-allow-passage/`, `evidence/manual-deny-passage/`), the
decision-restart real-process capture
(`evidence/manual-decision-restart-fault/`), the actuator-failure real
BuildSim kill (`evidence/actuator-failure/`), and the full Docker Compose
verification pass (`evidence/final-verification/`). Correctness was
checked against the real binary repeatedly; *performance* numbers never
were.
What not to claim: that any latency or throughput number in this report
reflects real-BuildSim performance — none do.
Follow-up: "Why didn't you benchmark against real BuildSim?" — honest
answer: not prioritized within this project's scope; stated as a named
gap (§13), not discovered by the examiner.

**D3. "Is the stress benchmark measuring your system, or measuring
itself?"**
Answer: Measuring the system, and this was specifically checked, not
assumed — the first sweep used a benchmark client that didn't drain
HTTP response bodies before closing connections (a classic Go
connection-reuse bug that would have been a tooling artifact). The
client was fixed (bodies drained, its own transport given a 2000-
connection pool so it could never itself be the limit) and the sweep was
re-run; the breaking point didn't move, and the actual error originates
in the *Decision Service's own log*, not the benchmark tool's. That's
the basis for "confirmed, not assumed."
What not to claim: that every possible benchmark-tool confound was ruled
out — only the one identified (undrained bodies) was found and fixed;
it's the most likely one given Go's HTTP client behavior, not the only
conceivable one.
Follow-up: "What's a confound you didn't check?" — honest, prepared
answer: the benchmark and the system under test run on the same
machine, competing for the same CPU/file-descriptor limits — no attempt
was made to isolate them (e.g., separate machines, cgroup limits) to
rule out the host itself being a shared confound.

**D4. "Is 'recovery time' defined correctly — restart issued to first
correct decision, or something narrower?"**
Answer: Precisely as named:
`t_recovered - t_restart_begin`, where `t_restart_begin` is when the
test issues the restart and `t_recovered` is when the first
*subsequent* access attempt produces a correct, confirmed ALLOW/UNLOCK
(`test/e2e/decision_restart_test.go`, `tRestartBegin`/`tRecovered`).
This measures the Decision Service's own readiness, not network
propagation to a client, not operator reaction time.
What not to claim: that this equals "time the door was inaccessible" —
a locked door *stays safely locked* the whole outage (REQ-RES-001); this
metric is about resuming correct *decision-making*, not about a user-
facing outage window, and conflating the two overstates what was
measured.
Follow-up: "Why does TOTAL OUTAGE (§evaluation-plan) differ from
RECOVERY TIME?" — because total outage includes the test's own fixed
setup/poll-interval overhead between steps (confirmed ~244 ms
consistently across runs in the raw log), which is test harness pacing,
not system behavior — recovery time is the cleaner, confound-isolated
number, and the report leads with it for that reason.

**D5. "n=200 for latency, n=10 for recovery time, one run for the
stress sweep. Are these sample sizes actually defensible?"**
Answer, honestly, item by item: n=200 for latency is a reasonable sample
for a tight, low-variance in-process measurement (max 0.928 ms, p95
0.601 ms — low spread, 200 points resolve the percentiles adequately).
n=10 for recovery time is thin for a tail-latency claim but adequate for
the actual claim made (a mean and a range, not a percentile) — the
report doesn't claim a p95 recovery time, deliberately, because 10
points can't support one credibly. The stress sweep is **one run per
concurrency level**, 2 seconds each — this is the weakest sample size in
the whole evaluation: a single 2-second window per level, no repeated
trials, no confidence interval.
What not to claim: that the stress numbers (especially the exact
breaking point at "concurrency 100" or the 3000-level recovery numbers)
are precise to the level implied by reporting them to the req/s and ms —
they're one-shot measurements of a genuinely noisy phenomenon
(connection-pool exhaustion timing).
Follow-up: "If you re-ran the stress sweep right now, would you expect
identical numbers?" — honest answer: the *qualitative* result (collapse
between 50 and 100, partial recovery at 3000) would very likely
reproduce, because the root cause is a hard architectural limit, not
noise — but the *exact* numbers (1687 req/s at 3000, specifically) would
likely vary run to run, since only one run was ever taken.

**D6. "What important failure domains remain completely untested?"**
Answer, the honest full list: (1) Card Reader, OccupancySensor,
DoorStateSensor crash/restart — architecturally reasoned to behave like
Decision/Actuator but never independently exercised; (2) delayed or
out-of-order message delivery — no mechanism exists to even produce
this condition in testing, since nothing in the pipeline reorders or
delays; (3) duplicate or corrupted *observation* data specifically (as
opposed to duplicate *commands*, which REQ-DIST-007 does cover); (4) the
Decision Service losing state mid-unlock and restarting with an
orphaned physically-unlocked door (§B2 above); (5) concurrent multi-door
contention (only one door exists to test with); (6) BuildSim failure's
effect on the Simulator and DoorStateSensor specifically, as opposed to
the Actuator (§B3).
What not to claim: that these are minor gaps — several of them (2, 3,
6) are exactly the kind of fault the course's own Dimension B/D grade-4
examples name explicitly ("delayed messages," "invalid sensor data").
Say the list completely and let it stand; don't minimize it.
Follow-up: "Of those six, which worries you most?" — a defensible,
honest answer: (4), the orphaned-unlock-on-restart gap, because it's the
one closest to an actual safety property (a door left unlocked with
nothing watching it) rather than a data-quality or coverage gap.

**D7. "Does your conclusion claim more than your tests actually cover?"**
Answer: Report §14 says "yes, within the tested prototype envelope, with
explicitly identified limits" — that hedge is load-bearing and should be
defended as written, not walked back under pressure. The "safe" claims
in §14 (invalid card never unlocks, DENY never produces UNLOCK, anomaly
can't override denial, fail-closed, no false CONFIRMED, no duplicate
physical effect) are each tied to a specific test named earlier in the
report — none of the six is asserted without a direct test backing it.
What not to claim under pressure: don't extend "robust to failure" to
failure modes not in §13's list (D6 above) just because the overall tone
of the conclusion is confident — the conclusion's hedges exist precisely
so this doesn't happen, so lean on them explicitly rather than
improvising reassurance.
Follow-up: "If I asked you to bet money on this system in a real
building tomorrow, what would you refuse to guarantee?" — strong, honest
answer: the orphaned-unlock-on-Decision-restart gap (D6/B2) and the
complete absence of duplicate/stale observation handling (C4/C5) — name
these two specifically, unprompted, as the two things you would not
stake a real deployment on without further work.

