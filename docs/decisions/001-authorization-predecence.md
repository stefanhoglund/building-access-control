# ADR-001: Authorization Precedence

## Status

Accepted

## Context

The access-control system supports permissions assigned both to individual Users and to Roles.

More than one permission may apply to the same access attempt. For example, a User may inherit an ALLOW permission from a Role while also having a User-specific DENY override.

The authorization result must therefore be deterministic and reproducible.

The system also follows a fail-closed policy when no applicable permission exists.

## Decision

Authorization shall use the following precedence:

```text
1. invalid credential        -> DENY
2. explicit user DENY        -> DENY
3. explicit user ALLOW       -> ALLOW
4. matching role DENY        -> DENY
5. matching role ALLOW       -> ALLOW
6. no applicable permission  -> DENY
```

User-specific permissions take precedence over Role permissions.

Within the same subject level, DENY takes precedence over ALLOW.

Permission constraints such as controlled boundary, direction, valid day, and valid time must match before the permission participates in precedence evaluation.

Authorization is evaluated using model time.

## Alternatives Considered

### Role DENY always overrides User ALLOW

This would provide a more restrictive policy but would prevent User-level exceptions from overriding Role policy.

### Explicit numeric permission priority

Each Permission could include a priority value.

This is more flexible but introduces additional configuration complexity and makes policy behavior harder to explain and validate.

### First matching rule wins

Permission ordering could determine the result.

This is simple to implement but makes behavior depend on configuration order and is less explicit.

## Consequences

The authorization policy is deterministic and easy to test.

User-specific exceptions are supported.

Conflicting permissions can be resolved without relying on configuration ordering.

The fixed precedence reduces policy flexibility compared with explicit priority rules, but this is acceptable for the initial system.