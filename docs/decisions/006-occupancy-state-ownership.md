# ADR-006: Occupancy State Ownership

## Status

Accepted

## Context

The system distinguishes between:

- simulated physical truth
- BuildSim visualization/current representation
- sensor observation

Occupancy is produced by simulated occupant movement.

The OccupancySensor must be able to disagree with underlying truth during fault, delay, stale-data, and anomaly experiments.

If the OccupancySensor itself owns occupancy truth, sensor errors cannot be modeled independently.

BuildSim is used to represent current building state visually, but the project's occupant simulation controls movement behavior.

## Decision

The Occupant and Movement Simulator shall be authoritative for simulated occupant location and physical occupancy truth.

When a SimulatedPerson successfully moves between rooms, the simulator updates its authoritative occupancy state.

BuildSim may receive mirrored occupancy state for visualization or shared current-state representation, but this does not make sensor observations authoritative.

OccupancySensor processes shall observe simulated occupancy truth and emit independent occupancy observations.

These observations may differ from truth because of:

- delay
- staleness
- missing observations
- injected sensor error
- process failure

The system therefore distinguishes:

```text
Simulator occupancy truth
        |
        +----> BuildSim occupancy representation
        |
        +----> Occupancy Sensor
                   |
                   v
             OccupancyObservation
```

Authorization shall not depend on occupancy unless explicitly specified by a future policy.

Occupancy may be used as contextual input for anomaly detection.

## Alternatives Considered

### BuildSim owns occupancy truth

The simulator could update BuildSim and all components could treat BuildSim occupancy as authoritative.

This is possible, but it couples simulation truth directly to the visualization/shared-state system and makes sensor-truth separation less explicit.

### OccupancySensor owns occupancy state

The sensor process could directly maintain and publish occupancy.

This incorrectly makes the measuring device authoritative for the physical quantity it is intended to observe.

### Event/Data Service derives occupancy

Occupancy could be reconstructed from movement events.

This is useful analytically but would make historical event processing the source of current simulated truth.

## Consequences

Simulation truth and sensor measurement remain explicitly separate.

Occupancy-sensor faults and data-quality problems can be evaluated realistically.

The simulator must expose occupancy truth through a defined mechanism that OccupancySensor processes can observe.

BuildSim's role must be described narrowly enough that it is not interpreted as authoritative for all occupancy semantics.

The corresponding requirement and architecture language should distinguish physical/simulated occupancy truth from BuildSim's visualization or mirrored occupancy state.