# ADR-004: Versioned System Configuration

## Status

Accepted

## Context

The system requires authoritative configuration for:

- Users
- AccessCards
- Roles
- Permissions
- SecurityZones
- ControlledDoors
- device identities
- device-to-door bindings
- controlled directions
- BuildSim identifiers

These concepts are required by multiple runtime components.

The architecture should avoid creating a new configuration microservice unless dynamic configuration management becomes necessary.

The configuration must also be reproducible so that experiments can be repeated using the same security topology and authorization policy.

## Decision

The system shall use a versioned System Configuration as the authoritative source for static access-control and device configuration.

The configuration shall include, at minimum:

```text
Users
AccessCards
Roles
Permissions
SecurityZones
ControlledDoors
Device Definitions
Device Bindings
```

The configuration shall be stored as repository-managed machine-readable files such as YAML or JSON.

Runtime processes shall load only the configuration required for their responsibilities.

The configuration shall have a version or stable configuration identifier that can be recorded with experiment evidence.

BuildSim runtime state shall not replace the System Configuration as the source of security intent.

## Alternatives Considered

### Dedicated configuration service

A runtime service could expose all configuration through an API.

This supports dynamic changes but introduces another process, network dependency, and failure mode.

### Duplicate configuration per process

Each process could maintain its own independent configuration.

This reduces runtime dependencies but risks inconsistency and duplicated authoritative state.

### Infer security configuration from BuildSim

Zones, permissions, and controlled boundaries could be derived from BuildSim topology.

BuildSim describes physical topology but does not contain the project's authorization and security intent.

## Consequences

Configuration is simple, reproducible, inspectable, and version-controlled.

Multiple processes may load the same configuration, but there remains one authoritative definition.

Runtime configuration changes are not initially supported dynamically.

If dynamic updates later become necessary, the architecture may evolve toward a managed configuration service.