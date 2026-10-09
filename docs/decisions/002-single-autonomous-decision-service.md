# ADR-002: Single Autonomous Decision Service

## Status

Accepted

## Context

The autonomous decision path contains several logically distinct functions:

- authorization
- anomaly detection
- decision policy
- runtime assurance

These functions could be deployed as separate network services or colocated within one independently deployable process.

The course requires an independently deployable autonomous decision-making component but does not require every internal decision function to be deployed separately.

Additional service boundaries would introduce more network communication, failure modes, configuration, deployment complexity, and operational overhead.

## Decision

Authorization, anomaly detection, decision policy, and runtime assurance shall initially be implemented as logically separate modules within a single Autonomous Decision Service process.

The internal separation shall remain explicit in the code and design.

The Autonomous Decision Service shall remain independently deployable and restartable from sensor and actuator processes.

Internal module boundaries shall not require network communication.

## Alternatives Considered

### Separate authorization service

Authorization could run as an independent service.

This would provide stronger process isolation but introduces another runtime dependency on the critical access path.

### Separate anomaly-detection service

Anomaly detection could run independently.

This would allow independent scaling or replacement, but the current system does not require either.

### Separate runtime-assurance service

Runtime assurance could be isolated as a separate process.

This would strengthen architectural separation but adds a network boundary immediately before actuation and increases complexity without a demonstrated need.

## Consequences

The architecture has fewer runtime services and simpler deployment.

Authorization, anomaly detection, decision policy, and runtime assurance remain conceptually and structurally distinct.

A failure of the Autonomous Decision Service affects all colocated decision functions.

If later evaluation requires independent failure injection, scaling, or deployment of one decision function, the service boundary may be revisited.