# Design

## Capture point

`recordCyberPolicyIfMarked` already runs after the upstream response has been
classified as `cyber_policy` and has access to the original request body for
Responses, Chat Completions, Messages, and per-turn WebSocket traffic. It will
build immutable evidence before starting the existing asynchronous recording
goroutine.

## Normalization

The security-audit protocol parser is the canonical source of client-controlled
text. A reusable conversation-evidence extractor will preserve source order and
role labels while excluding binary image data and unrelated JSON fields. The
hash is calculated over the complete normalized text; the stored snapshot is
bounded and then passed through the existing content-moderation secret
redactor.

The persisted fields are:

- `protocol`, `audit_stage`, and `turn_number`;
- `input_hash`, `input_length`, `message_count`, and `input_truncated`;
- short `input_excerpt` for lists and bounded `input_snapshot` for detail.

Legacy rows receive safe defaults and remain distinguishable by their empty
hash and snapshot.

## Reliability and side effects

The evidence is attached to the same `ContentModerationLog` insert as the
existing cyber event, so no partially linked detail row is possible. Existing
feature gates, violation counting, email, usage, ops logging, session blocking,
and client responses remain unchanged.

Extraction failures are fail-open for recording: the base cyber event is still
stored with empty evidence fields. No error path may expose request text in
application logs.

## Privacy

Only normalized textual conversation content is retained. Secrets are redacted
before PostgreSQL insertion, snapshots are limited to 12,000 runes, list
previews remain limited to 240 runes, and image/base64 payloads are excluded.
Access and cleanup continue to use the existing administrator risk-control
workflow and hit-retention policy.
