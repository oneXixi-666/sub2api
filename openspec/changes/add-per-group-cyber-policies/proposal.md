# Add per-group cyber policies

## Why

The current cyber_policy configuration only selects which groups receive one
shared enforcement behavior. Administrators need a collect-only default while
individual groups independently control session blocking, violation counting,
notification, and account bans.

## What changes

- Replace the binary group selector with a default Cyber policy plus complete
  per-group policy overrides.
- Keep every official cyber_policy hit reviewable regardless of proactive
  moderation group or model filters.
- Isolate Cyber violation counters by user and group, while retaining the
  existing whole-account ban action.
- Persist the resolved policy source and snapshot with each audit row.
- Preserve v0.2.2 configurations by converting the legacy selector at load.

## Impact

This changes content-moderation configuration JSON, cyber session-block gates,
Cyber account-ban counting, audit-log persistence, and the Risk Control admin
editor. Existing configuration remains readable and behaviorally compatible.
