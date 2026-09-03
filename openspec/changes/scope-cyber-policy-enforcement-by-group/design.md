# Design

## Separate scopes

The content-moderation configuration gains `cyber_policy_enforce_all_groups`
and `cyber_policy_enforce_group_ids`. They are independent from `all_groups`
and `group_ids`, which continue to control proactive content moderation only.

Old configuration JSON is unmarshaled over defaults, so missing cyber scope
fields resolve to all groups and preserve existing enforcement behavior.

## Runtime enforcement

A cached content-moderation service decision determines whether an API key's
group is in the cyber enforcement scope. The handler applies that decision at
both session-block lookup and session-block write points for Responses, Chat
Completions, Messages, and Responses WebSocket traffic. Configuration errors
fail open and do not locally reject traffic.

The upstream `cyber_policy` result for the current request is immutable: group
scope only controls local follow-up action after the provider has rejected it.

## Collection-only records

`RecordCyberPolicyEvent` no longer drops events merely because their group is
outside the ordinary content-moderation scope. Model filtering and the global
risk-control switch remain in force.

Each new row stores `cyber_policy_mode` as `enforce` or `collect_only`. An
enforced row retains the existing violation/ban and email flow. A collect-only
row is persisted without those local side effects and is always excluded from
future rolling violation counts.
