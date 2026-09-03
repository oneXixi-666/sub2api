# cyber-policy-group-policies Specification

## ADDED Requirements

### Requirement: Every official Cyber hit remains reviewable

While the Risk Control master switch is enabled, the system SHALL persist every
official upstream `cyber_policy` hit regardless of proactive moderation group,
model, mode, sampling, or API configuration.

#### Scenario: Proactive model filter excludes the request model

- **WHEN** an excluded model receives an official upstream `cyber_policy` result
- **THEN** the system stores the conversation evidence and resolved Cyber policy snapshot

### Requirement: Groups resolve independent Cyber policies

The system SHALL apply a group override when present and otherwise apply the
default Cyber policy.

#### Scenario: Group override is collect-only

- **WHEN** a group with a collect-only override receives a Cyber hit
- **THEN** the hit is stored and no local session block, count, email, or ban action runs

#### Scenario: Group override enables selected actions

- **WHEN** an enforcing group policy enables session blocking and email but disables counting
- **THEN** the session is blocked for that policy TTL and a hit email is available, but no violation or account-ban decision runs

### Requirement: Cyber ban counters are isolated by group

The system SHALL count enforcing Cyber violations for the same user and same
group within the resolved policy window.

#### Scenario: Another group has Cyber violations

- **WHEN** a user's current group reaches its configured threshold only after including hits from another group
- **THEN** the system does not ban the account until the current group independently reaches its threshold

### Requirement: Account bans remain account-wide

An automatic ban triggered by any enforcing group policy SHALL disable the
whole user account and invalidate cached authentication.

#### Scenario: A group reaches its automatic-ban threshold

- **WHEN** the same user and group reach the configured Cyber threshold
- **THEN** the system disables the whole user account and invalidates that user's authentication cache

### Requirement: Historical action basis is immutable

Each new Cyber audit row SHALL store whether the default or group override was
used and the complete effective policy snapshot.

#### Scenario: Administrator changes a group policy later

- **WHEN** a previously recorded Cyber row is reviewed
- **THEN** it still exposes the policy that was effective when the hit occurred

### Requirement: Legacy selectors retain their behavior

Configurations without the new policy fields SHALL map the old all-groups or
selected-groups selector to an equivalent default plus overrides policy model.

#### Scenario: A legacy selected-groups configuration is loaded

- **WHEN** the old configuration selects groups 7 and 9 for enforcement
- **THEN** the system resolves a collect-only default and enforcing overrides for groups 7 and 9
