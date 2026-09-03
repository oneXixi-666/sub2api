# Capture cyber_policy audit evidence

## Why

Upstream `cyber_policy` hits are recorded today, but the row contains only the
provider error. The request conversation is discarded, so historical rows
cannot be reviewed or used to derive candidate risk phrases.

## What changes

- Capture a bounded, role-preserving text snapshot for every newly recorded
  upstream `cyber_policy` hit.
- Persist protocol, request stage, WebSocket turn, normalized input hash,
  original length, segment count, and truncation state with the existing risk
  control row.
- Show the detailed snapshot and capture metadata in the existing risk-control
  detail dialog.
- Keep `matched_keyword` empty unless a deterministic local keyword rule
  actually matched. Captured text is evidence, not proof of the provider's
  internal trigger.

## Non-goals

- Guessing or backfilling keywords for historical rows.
- Automatically adding inferred phrases to `blocked_keywords`.
- Claiming that request text alone explains an output-time provider safeguard.
- Adding a second moderation pass, user ban, or notification side effect.
