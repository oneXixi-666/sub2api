# cyber-policy-enforcement Specification

## ADDED Requirements

### Requirement: Cyber enforcement has an independent group scope

The system SHALL support all-groups and selected-groups cyber enforcement
without changing the group scope used by proactive content moderation.

#### Scenario: Existing configuration has no cyber scope fields

- **WHEN** an existing content-moderation configuration is loaded
- **THEN** cyber enforcement defaults to all groups and preserves prior behavior

#### Scenario: A selected group continues a blocked session

- **WHEN** the global cyber session-block switch is enabled and a request group is in the cyber enforcement scope
- **THEN** cyber session-block lookup and write behavior remains enabled for that group

#### Scenario: An unselected group continues a blocked session

- **WHEN** a request group is outside the cyber enforcement scope
- **THEN** the gateway does not read or write a local cyber session block for that group

### Requirement: Cyber evidence collection continues outside enforcement scope

The system SHALL persist new upstream `cyber_policy` events for groups outside
the cyber enforcement scope as `collect_only`, subject to the global
risk-control switch and model filter.

#### Scenario: An unselected group receives an upstream cyber_policy result

- **WHEN** the provider rejects a request from an unselected group with `cyber_policy`
- **THEN** the audit row is stored with `cyber_policy_mode=collect_only`, is permanently excluded from violation totals, and no local ban or enforcement-email side effect runs

#### Scenario: A selected group receives an upstream cyber_policy result

- **WHEN** the provider rejects a request from a selected group with `cyber_policy`
- **THEN** the audit row is stored with `cyber_policy_mode=enforce` and existing configured enforcement side effects remain available

### Requirement: The administrator UI explains the enforcement boundary

The system SHALL let administrators choose the cyber enforcement groups and
SHALL explain that all groups are still collected and that the current upstream
rejection cannot be undone.

#### Scenario: Administrator selects specific enforcement groups

- **WHEN** the administrator saves selected-groups mode with one or more group IDs
- **THEN** the saved configuration is restored accurately and the UI identifies non-selected groups as collection-only
