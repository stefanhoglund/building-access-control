# Final Verification Run

Run date (UTC): 2026-10-10, starting 05:29:37Z.
Git state at run start: `59dff71d321c19510331ddfd8b10780346dc77b8` ("Complete
manual successful-passage CPS validation") plus an uncommitted working tree
(51 changed/untracked paths — everything built since that commit: the full
vertical slice, fault experiments, performance/anomaly work, named scenarios,
and the Docker Compose deployment). Per CLAUDE.md/project convention, nothing
is committed automatically; the working tree as it stands at the end of this
pass is the submission candidate.

## Files in this directory

| File | Contents |
|---|---|
| `01-regression.log` | `gofmt`, `go vet`, `go test ./...`, `-race`, `-tags=integration`, `jq` validation of all three config files |
| `02-docker-build.log` | `docker compose config` + `docker compose build` |
| `03-docker-up.log` | `docker compose up -d` + `docker compose ps` (all 9 containers reached running/healthy) |
| `04-scenario-checks.log` | Initial door state, dashboard reachability, `authorized-passage`, `unauthorized-access`, `after-hours-access`, `rapid-repeat-access` run against the live stack |
| `05-decisions-check.log` | The resulting `AccessDecision` records for the four scenarios above, confirming correct authorization/anomaly/final_action values |
| `06-restart-and-persistence.log` | `docker compose restart decision` + fresh access; evidence line counts before/after a full `docker compose restart`; **this run is where the door-seed race (see below) was first observed** |
| `07-restart-recovery-clean.log` | Diagnostic follow-up after 06: settling the pending queue and confirming decision-restart recovery in isolation |
| `08-door-seed-fix-verification.log` | First repro of the door-seed DNS race after the `compose.yaml` fix (ambiguous due to accumulated container logs) |
| `09-actuator-failure-and-dashboard.log` | Clean rebuild, a second unambiguous repro of the race (confirmed fixed), actuator stop/restore, dashboard check |

## Result

All required Phase-3 checks passed:

- `gofmt -l .` clean, `go vet ./...` clean, `go test ./...` and `-race` green,
  `-tags=integration` green against the real BuildSim binary.
- All three `config/*.json` files are valid JSON (`jq empty`).
- `docker compose config` validates; `docker compose build` builds one
  `access-control:latest` image shared by all 8 project services plus
  `buildsim`'s own (unmodified) image.
- `docker compose up -d`: all 9 containers reach running/healthy.
- door-1 exists, LOCKED, CLOSED at a fresh start.
- `authorized-passage`: full ALLOW → UNLOCK → OPEN → occupancy change → CLOSE
  → RELOCK_AFTER_PASSAGE → LOCKED, confirmed via BuildSim's own door-state
  read.
- `unauthorized-access`: DENY/`NO_PERMISSION`, door stays LOCKED.
- `after-hours-access`: ALLOW/`ROLE_ALLOW` + `ANOMALOUS`/`UNUSUAL_ACCESS_TIME`
  + `alert:true`, `final_action` unaffected by the anomaly classification.
- `rapid-repeat-access`: first attempt NORMAL, both repeats
  `ANOMALOUS`/`REPEATED_ATTEMPT`.
- `docker compose restart decision` followed by a fresh access attempt
  recovers correctly.
- Stopping `actuator`, presenting a card: `UNLOCK` command recorded, **no**
  outcome record follows it (no false `CONFIRMED`), door stays LOCKED;
  restarting `actuator` and presenting again recovers fully.
- Dashboard reachable at `http://localhost:8085/` (HTTP 200), renders live
  evidence.
- Evidence (`data/evidence.jsonl`, `evidence/scenarios/runs.jsonl`) persists
  and keeps growing correctly across both a single-service restart and a
  full `docker compose restart` of every container.

## Genuine finding from this pass: door-seed vs. bulk `docker compose restart`

**Observed** (`06-restart-and-persistence.log`): after a plain
`docker compose restart` (no arguments — every container, including
`buildsim` and `door-seed`, restarted together), `GET /api/doors` against
BuildSim returned `null`: door-1 had not been re-created. `docker compose
logs door-seed` showed `curl: (6) Could not resolve host: buildsim`.

**Root cause, confirmed not assumed**: `depends_on: condition:
service_healthy` is a `docker compose up` behavior; a bulk `docker compose
restart` restarts every container at roughly the same time without waiting
for that condition. `door-seed`'s one-shot `curl` ran before Compose's
embedded DNS had (re-)registered the `buildsim` hostname for the
freshly-restarting `buildsim` container, so its single attempt failed outright.
This is specific to restarting *every* container in the stack in one bulk
command; restarting a single project service (`decision`, `actuator`, as
exercised throughout this pass and the prior Docker verification) is
unaffected, because `buildsim` itself was not also restarting at that moment.

**Fix applied** (`compose.yaml`, `door-seed` service): added
`--retry 10 --retry-all-errors --retry-delay 1` to the `curl` command. This
is the smallest correction that addresses the actual failure (a transient
DNS/connection race during a bulk restart), does not change normal startup
behavior (a fresh `docker compose up` still waits for `buildsim` healthy
before `door-seed` ever runs), adds no new service, and touches no
application code.

**Verified fixed** (`08-door-seed-fix-verification.log`,
`09-actuator-failure-and-dashboard.log`): the identical `docker compose
restart` (all containers) was repeated twice more after the fix, on a freshly
recreated stack; both times `door-seed`'s log shows the same transient DNS/
connection failures followed by a successful retry, and `GET /api/doors`
confirms door-1 is LOCKED/CLOSED immediately afterward both times.

**Scope of this finding**: this affects only the specific operator action
"restart every container in the stack simultaneously via a single bulk
`docker compose restart`." It does not affect: starting the stack fresh
(`docker compose up`/`make docker-up`), restarting any single project
service, or any safety invariant (no unlock was ever produced from stale/
missing door state — the symptom was `door-1` simply not existing, which
BuildSim would reject any actuator command against, not silently miscommand).
It is recorded here, in `docs/deployment.md`, and in
`docs/evaluation-results.md` as a found-and-fixed deployment-robustness
issue, not swept under the rug.
