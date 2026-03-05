# Model Preferences Routing

## Purpose

Define how `omp` resolves and applies model mappings for OpenCode targets.

## Routing Sources

`omp` supports two routing sources in `omp-slots.json`:

1. `target_slots` (target -> slot)
2. `target_models` (target -> model, direct)

## Resolution Order

When applying mappings, each target resolves in this order:

1. If `target_models[target]` is non-empty, use that model.
2. Else if `target_slots[target]` exists and slot has a non-empty model, use slot model.
3. Else leave target unchanged.

This guarantees deterministic precedence:

`direct model > slot model > unchanged`

## TUI Controls

### Assignments view

- `s` assign/clear slot
- `m` assign/clear direct model
- `a` apply mappings

### Slots view

- `enter` set slot model
- `r` rename slot
- `n` add slot
- `x` remove slot

Removing a slot clears any `target_slots` entries pointing to that slot.
