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
  }
}
```

Each target (agent or command) maps directly to a model ID.

## Resolution

When applying preferences, each target resolves as:

1. If `target_models[target]` is non-empty, write that model to `opencode.json`.
2. Else leave target unchanged.

Only targets that already exist in `opencode.json` are written to.

## TUI Sections

The TUI groups targets into three sections:

- **Agents** — visible, user-facing agents
- **Sub-agents** — agents with `hidden: true` (e.g. plugin sub-agents like `adv-researcher`). Not shown in OpenCode's Tab-cycle but configurable here.
- **Commands** — slash commands with model overrides

## TUI Controls

- `enter` / `m` — pick a model for the selected agent/command
- `d` — clear model assignment
- `a` — apply all preferences to `opencode.json`
- `/` — filter the list
- `q` — quit
