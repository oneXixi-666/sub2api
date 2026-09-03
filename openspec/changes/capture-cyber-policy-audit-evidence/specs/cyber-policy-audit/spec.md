# cyber-policy-audit Specification

## ADDED Requirements

### Requirement: New upstream cyber_policy hits retain reviewable input evidence

When the gateway receives an upstream `cyber_policy` result and the existing
risk-control scope permits the event to be recorded, the system SHALL persist a
bounded, secret-redacted, role-preserving textual snapshot with the same audit
row.

The row SHALL also contain the protocol, request stage, WebSocket turn number
when applicable, normalized input hash, original normalized length, segment
count, and truncation indicator.

#### Scenario: HTTP request is blocked upstream

- **WHEN** a Responses, Chat Completions, or Messages request receives an upstream `cyber_policy` result
- **THEN** the corresponding content-moderation row contains the normalized conversation evidence and HTTP stage metadata

#### Scenario: WebSocket turn is blocked upstream

- **WHEN** a WebSocket turn receives an upstream `cyber_policy` result
- **THEN** the row contains evidence for that turn and records its turn number and stage

#### Scenario: Evidence cannot be extracted

- **WHEN** the request has no supported textual content or evidence extraction fails
- **THEN** the base cyber event is still stored with empty evidence fields

### Requirement: Captured evidence does not assert provider keyword attribution

The system SHALL NOT populate `matched_keyword` from conversation text merely
because the provider returned `cyber_policy`, and SHALL NOT mutate
`blocked_keywords` as a side effect of evidence capture.

#### Scenario: Conversation contains security terminology

- **WHEN** captured evidence contains terms such as `shell`, `payload`, or `exploit` without a deterministic local keyword hit
- **THEN** those terms remain review evidence and are not recorded as matched provider keywords or added to the local block list

### Requirement: Existing cyber behavior remains unchanged

Evidence capture SHALL NOT add a moderation call, delay or rewrite the client
response, trigger duplicate violation counting or emails, change usage billing,
or change session-block behavior.

#### Scenario: Evidence is attached to an existing cyber event

- **WHEN** a cyber event is recorded with conversation evidence
- **THEN** the existing client response, billing, notification, violation-counting, and session-block flows execute with their prior semantics
