# ADR-003: Model Time Ownership and Distribution

## Status

Accepted

## Context

The system uses simulated time for:

- permission windows
- day-of-week rules
- occupant schedules
- behavior profiles
- scenario timing
- anomaly context
- the configured door relock period
- reproducible experiments

The system also uses wall-clock time for distributed-system concerns such as network latency, process health, timeouts, and recovery measurements.

Independently running processes must evaluate simulated events against a consistent model-time reference.

If individual components derive simulated time independently from wall-clock time, accelerated, paused, or replayed simulations may become inconsistent.

## Decision

The Occupant and Movement Simulator shall be the authoritative owner of model time.

Model time shall be represented as an absolute simulated date and time.

Simulation-originated events shall include `model_time`.

Downstream records derived from a simulation event shall preserve the model time associated with the triggering event.

Components shall not independently infer model time from their local wall clock.

Wall-clock time shall remain separate and shall be used for runtime concerns including:

- network timeout
- process liveness
- execution latency
- recovery-time measurement
- operational health checks

The mechanism for retrieving current model time outside the context of an existing event will be defined in the relevant interface contract if required.

## Alternatives Considered

### Derive model time independently from wall clock

Each process could calculate model time using a common start time and acceleration factor.

This avoids distributing timestamps but risks drift and inconsistent behavior across process restarts or pauses.

### Dedicated model-time service

A separate service could own and publish simulation time.

This creates an additional distributed component and failure mode without a current requirement for independent clock management.

### BuildSim owns model time

BuildSim could theoretically provide the clock.

The current project design treats the student simulator as the owner of scenario behavior and simulation progression, making it the more natural model-time authority.

## Consequences

All distributed components share a consistent simulated time reference through event propagation.

Model time and wall-clock time remain semantically distinct.

The simulator becomes a dependency for progression of simulation time.

Safety behaviors expressed in model time may require a separate wall-clock safety backstop if simulator time stops advancing. That behavior remains a separate design decision.