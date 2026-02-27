# omp — OpenCode Model Preferences

A standalone TUI for managing per-agent and per-command model preferences in [OpenCode](https://github.com/anomalyco/opencode).

## Why

OpenCode supports setting `model` overrides for individual agents and slash commands in `opencode.json`, but there's no built-in UI for it. `omp` gives you a filterable picker that discovers all your agents, commands, and registered models, then writes the preference directly to your config.

## Install

Requires Go 1.23+.

```bash
# Clone and install to ~/.local/bin
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

Pull the latest changes and reinstall:

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

- `q`, `esc`, and `ctrl+c` are treated as graceful closes (exit code 0).
- Successful exits close the popup automatically.
- Non-zero exits stay visible in the popup for debugging.
- Preference changes apply on the next OpenCode agent/command invocation — no restart needed.

### Flow

1. **Target list** — Browse agents and commands grouped by type (primary agents, subagents, commands). Each entry shows its current model preference or `(default)`.
2. **Model picker** — Select a model from all providers in your config, or choose `(clear preference)` to remove the override.
3. **Write** — The preference is written to `~/.config/opencode/opencode.json` and the list refreshes.

### Keybinds

| Key | Action |
|-----|--------|
| `enter` | Select target / confirm model |
| `/` | Filter the current list |
| `ctrl+up` | Move agent up (custom primary agents only) |
| `ctrl+down` | Move agent down (custom primary agents only) |
| `esc` | Back to targets / quit from targets |
| `q` | Back to targets / quit from targets |
| `ctrl+c` | Same as `q` |

### Agent ordering

Custom primary agents can be reordered with `ctrl+up` / `ctrl+down`. The new order is written directly to `~/.config/opencode/opencode.json` and controls the **Tab-cycle order** in OpenCode (OpenCode uses JSON key insertion order for its agent cycle).

Built-in primary agents (`build` and `plan`) are shown as `[locked]` — their cycle positions are fixed by OpenCode and cannot be changed here. Custom primary agents always appear after them in the cycle.

## How it works

### Startup model refresh

On every launch, `omp` runs `opencode models --refresh` before loading the config. This ensures the model picker always reflects the latest models from your configured providers (including any newly added or removed models).

If the refresh fails, `omp` exits immediately with an actionable error:

| Failure | Error message |
|---------|--------------|
| `opencode` not in PATH | `opencode binary not found in PATH` |
| Non-zero exit (auth/network) | `opencode models --refresh failed: …` + command output |
| Timeout (>30s) | `opencode models --refresh timed out after 30s` |

### Discovery

- **Built-in agents**: `build`, `plan` (primary, locked); `general`, `explore` (subagent)
- **Markdown agents**: `~/.config/opencode/agents/*.md` and `.opencode/agents/*.md` — discovered first; `mode` from frontmatter takes precedence
- **JSON agents**: From `agent.*` keys in `opencode.json` (excludes system agents: `compaction`, `title`, `summary`); only adds agents not already defined by a markdown file. JSON `model` overrides are always applied regardless of which source defined the agent.
- **JSON commands**: From `command.*` keys in `opencode.json`
- **Markdown commands**: `~/.config/opencode/commands/*.md` and `.opencode/commands/*.md`
- **Models**: CLI-first discovery via `opencode models` output, with fallback to `provider.*.models` entries in `opencode.json` when the CLI is unavailable or returns no parseable models. This ensures the picker always shows OpenCode's runtime model registry.

> **Precedence note**: if an agent is defined in both a markdown file and `opencode.json`, the markdown `mode` wins. This means an agent with `mode: subagent` in its `.md` file correctly appears under Sub-Agents even if the JSON entry has no `mode` field.

### Config writes

Uses [tidwall/sjson](https://github.com/tidwall/sjson) for surgical JSON path updates that preserve existing formatting and comments. Sets `agent.<name>.model` or `command.<name>.model`. Clearing a preference deletes the key.

### Environment

| Variable | Purpose |
|----------|---------|
| `OPENCODE_CONFIG_DIR` | Override config directory (default: `~/.config/opencode`) |
| `OPEN_CHAD_OMP_POPUP_SIZE` | Override tmux popup size when launched via openchad (default: `80%x80%`) |

## Development

```bash
go test ./...     # run tests
go vet ./...      # lint
make build        # build binary
make clean        # remove binary
```
