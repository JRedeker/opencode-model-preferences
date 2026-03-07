# omp — OpenCode Model Preferences

A TUI for managing per-agent model preferences in [OpenCode](https://github.com/anomalyco/opencode).

Pick a model for each agent or command, then apply to write preferences to `opencode.json`.

## Install

Requires Go 1.24+.

```bash
git clone https://github.com/JRedeker/opencode-model-preferences.git
cd opencode-model-preferences
make install
```

Or build without installing:

```bash
make build
./omp
```

## Update

```bash
cd opencode-model-preferences
git pull
make install
```

## Usage

Run `omp` from any directory:

```bash
omp
```

### openchad tmux popup

When running inside openchad, press `Ctrl+b m` to open `omp` in a tmux popup (`display-popup -EE`, default 80%×80%). Requires tmux ≥3.2.

Override the popup size with `OPEN_CHAD_OMP_POPUP_SIZE`:

```bash
export OPEN_CHAD_OMP_POPUP_SIZE="90%x85%"   # percent
export OPEN_CHAD_OMP_POPUP_SIZE="120x40"    # absolute cells
```

### Flow

1. Browse agents, sub-agents, and commands — each shows its current model.
2. Press `enter` or `m` to pick a model for the selected target.
3. Press `d` to clear a model assignment.
4. Press `a` to apply all preferences to `opencode.json`.

The TUI groups targets into three sections:

- **Agents** — primary and user-facing agents (visible in OpenCode's Tab-cycle)
- **Sub-agents** — hidden agents used internally by plugins (e.g. `adv-researcher`, `adv-reviewer`). These have `hidden: true` in their config and don't appear in OpenCode's Tab-cycle, but you can still set model preferences for them here.
- **Commands** — slash commands with model overrides

### Keybinds

| Key | Action |
|-----|--------|
| `enter` / `m` | Pick model for selected agent/command |
| `d` | Clear model assignment |
| `a` | Apply preferences to opencode.json |
| `/` | Filter the list |
| `q` / `ctrl+c` | Quit |

> **Note:** Apply only writes to agents and commands that already have an entry in `opencode.json`. It does not create new agent entries. Clearing a model and applying removes the `model` key from `opencode.json` while preserving other fields.

## Config format

Preferences are stored in `~/.config/opencode/omp-preferences.json` (separate from `opencode.json`, which does not accept unknown keys):

```json
{
  "target_models": {
    "build": "anthropic/claude-opus-4",
    "plan": "anthropic/claude-opus-4",
    "general": "anthropic/claude-sonnet-4",
    "explore": "anthropic/claude-haiku-4"
  },
  "cleared_models": {
    "scout": true
  }
}
```

- `target_models` — maps each target to a model ID. Applying writes these to `opencode.json`.
- `cleared_models` — tracks targets whose model was explicitly cleared. Applying **removes** the `model` key from `opencode.json` for these targets (other fields like `mode` are preserved).

## How it works

### Startup model refresh

On every launch, `omp` runs `opencode models --refresh` before loading the config. This ensures the model picker always reflects the latest models from your configured providers.

If the refresh fails, `omp` exits immediately with an actionable error:

| Failure | Error message |
|---------|--------------|
| `opencode` not in PATH | `opencode binary not found in PATH` |
| Non-zero exit (auth/network) | `opencode models --refresh failed: …` + command output |
| Timeout (>30s) | `opencode models --refresh timed out after 30s` |

### Agent discovery

- **Built-in agents**: `build`, `plan` (primary, locked); `general`, `explore` (subagent)
- **Markdown agents**: `~/.config/opencode/agents/*.md` and project `.opencode/agents/*.md` — `mode` and `hidden` from frontmatter determine classification
- **JSON agents**: From `agent.*` keys in `opencode.json` (excludes system agents: `compaction`, `title`, `summary`). Agents with `"hidden": true` appear in the Sub-agents section.
- **Commands**: From `command.*` keys in `opencode.json` and markdown command files

### Model discovery

CLI-first discovery via `opencode models` output, with fallback to `provider.*.models` entries in `opencode.json`.

## Environment

| Variable | Purpose |
|----------|---------|
| `OPENCODE_CONFIG_DIR` | Override config directory (default: `~/.config/opencode`) |
| `OPENCODE_PROJECT_DIR` | Override project root used for `.opencode/agents` and `.opencode/commands` discovery |
| `OPEN_CHAD_OMP_POPUP_SIZE` | Override tmux popup size when launched via openchad (default: `80%x80%`) |

## Development

```bash
go test ./...     # run tests
go vet ./...      # lint
make build        # build binary
make clean        # remove binary
```
