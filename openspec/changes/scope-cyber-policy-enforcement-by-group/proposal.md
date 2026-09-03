# Scope cyber_policy enforcement by group

## Why

Cyber session blocking is currently global. Operators need to enable local
post-hit enforcement for selected groups while continuing to collect review
evidence from every other group.

## What changes

- Add an independent all-groups/selected-groups scope for cyber enforcement.
- Keep the existing all-groups behavior as the compatibility default.
- Continue storing upstream `cyber_policy` evidence outside the enforcement
  scope, marked as `collect_only`.
- Suppress local session blocking, violation/ban side effects, and enforcement
  email for collect-only groups.
- Make the distinction and the immutable upstream rejection explicit in the
  administrator UI.

## Non-goals

- Reversing or hiding the provider's rejection of the current request.
- Reusing the ordinary content-moderation scope as the cyber enforcement
  scope.
- Inferring provider trigger keywords from collected text.
