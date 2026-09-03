# Design

## Policy resolution

The configuration stores one complete `cyber_policy_default_policy` and an
ordered, unique `cyber_policy_group_policies` collection. A matching group
override replaces the default policy as a unit. Deleting an override restores
default inheritance.

The resolved policy separates these actions:

- session blocking and its TTL;
- group-local violation counting;
- per-hit email notification;
- whole-account automatic ban and threshold/window.

`collect_only` always resolves every action to disabled. Evidence collection is
not a policy option and remains controlled only by the Risk Control master
switch. The pre-existing system Cyber session-block switch remains an outer
safety switch; a policy cannot enable the cache-backed block when that master is
off.

## Compatibility

When the new default-policy field is absent, the parser converts the v0.2.2
selector. `enforce_all_groups=true` becomes an enforcing default. A selected
group list becomes a collect-only default plus enforcing overrides. Existing
global ban settings seed the converted enforcement policies.

The legacy fields remain in the DTO for one-way client compatibility, but new
policy fields are authoritative.

## Audit and counting

Every new Cyber row stores `cyber_policy_source` and a versioned JSON policy
snapshot. Historical rows receive an empty snapshot because their exact former
policy cannot be reconstructed reliably.

Cyber automatic-ban counts include only enforcing Cyber rows whose resolved
snapshot enabled counting, for the same user and the same nullable group. Other
content-moderation violations keep their existing global counting behavior.
The resulting ban still disables the entire user account and invalidates its
authentication cache.
