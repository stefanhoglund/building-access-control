# ADR-005: Control Path Through the Event and Data Service

## Status

Accepted

## Context

Sensor observations must be:

- validated
- timestamped
- checked for duplicates
- checked for malformed data
- made available to the Autonomous Decision Service
- retained for replay and evaluation

Two broad control-path designs are possible:

```text
Sensor -> Decision Service
Sensor -> Data Pipeline
```

or:

```text
Sensor -> Event/Data Service -> Decision Service
```

Routing observations through the Event and Data Service provides a single validation and evidence boundary but places that service on the critical access-control path.

## Decision

The primary sensor-to-decision control path shall pass through the Event and Data Service.

The logical path is:

```text
Sensor Process
    -> Event and Data Service
    -> Autonomous Decision Service
    -> Door Actuator
```

The Event and Data Service is responsible for validating sensor observations before they reach the decision service.

Decision records, actuator commands, and actuator outcomes shall also be returned to the Event and Data Service for persistence in the Historical Evidence Store.

The Historical Evidence Store shall have a single logical writer through the Event and Data Service.

If the Event and Data Service is unavailable, new credential-based access attempts shall fail closed rather than bypassing validation.

## Alternatives Considered

### Direct sensor-to-decision path with parallel evidence path

Sensors could send directly to the Decision Service while independently writing to the data pipeline.

This may reduce decision latency but creates duplicate delivery paths and risks differences between evidence and decision inputs.

### Message broker between sensors and consumers

A broker such as MQTT could fan observations out to decision, storage, and dashboard consumers.

This improves decoupling but adds another infrastructure component and operational failure mode.

### Direct storage-first design

Sensors could persist first, with the Decision Service reading from storage.

This provides strong durability but would unnecessarily couple control latency to historical storage access.

## Consequences

All decision inputs pass through a common validation boundary.

Evidence and live-control data use consistent canonical observations.

The Event and Data Service becomes a single point of failure on the credential-based access path.

This is an intentional simplicity tradeoff and shall be evaluated through failure injection.

The exact acknowledgement, retry, and durability semantics remain to be defined in `data-contracts.md`.