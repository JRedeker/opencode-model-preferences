# Model Preferences Routing

## Purpose

Define how `omp` resolves and applies model preferences for OpenCode targets.

## Config

Preferences are stored in `omp-preferences.json`:

```json
{
  "target_models": {
    "build": "anthropic/claude-opus-4",
    "general": "anthropic/claude-haiku-4"
  },
  "cleared_models": {
    "scout": true
  }
}
```

- `target_models` — maps each target (agent or sub-agent) directly to a model ID.
- `cleared_models` — tracks targets whose model was explicitly cleared by the user.

## Resolution

When applying preferences, each target resolves as:

1. If `cleared_models[target]` is true, **delete** the `model` key from `opencode.json` (other fields preserved).
2. If `target_models[target]` is non-empty, write that model to `opencode.json`.
3. Else leave target unchanged.

Only targets that already exist in `opencode.json` are written to. Assigning a new model to a previously cleared target removes it from `cleared_models`.

## TUI Sections

The TUI groups targets into two sections:

- **Agents** — visible, user-facing agents
- **Sub-agents** — agents with `mode: subagent` or `hidden: true` (e.g. plugin sub-agents like `adv-researcher`). Not shown in OpenCode's Tab-cycle in the same way as primary agents, but configurable here.

Sub-agent mappings are sticky overrides. They do not automatically follow a main-agent model change. Clearing a sub-agent mapping returns it to inherited/default OpenCode routing.

## TUI Controls

- `enter` / `m` — pick a model for the selected agent
- `d` — clear model assignment
- `D` — clear all sub-agent overrides
- `a` — apply all preferences to `opencode.json`
- `/` — filter the list
- `q` — quit
