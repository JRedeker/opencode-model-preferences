# omp - OpenCode Model Preferences

A standalone TUI for managing per-agent and per-command model preferences in [OpenCode](https://github.com/anomalyco/opencode).

## Why

OpenCode supports setting `model` overrides for individual agents and slash commands in `opencode.json`, but there's no built-in UI for it. `omp` gives you a filterable picker that discovers all your agents, commands, and registered models, then writes the preference directly to your config.

## Install

Requires Go 1.23+.

```bash
# Clone and install to ~/.local/bin
git clone https://github.com/anomalyco/opencode-model-preferences.git
cd opencode-model-preferences
make install
```

Or build without installing:

```bash
make build
./omp
```

## Usage

Run `omp` from any directory:

```bash
omp
```

### Flow

1. **Target list** - Browse agents and commands grouped by type (primary agents, subagents, commands). Each entry shows its current model preference or "(default)".
2. **Model picker** - Select a model from all providers in your config, or choose "(clear preference)" to remove the override.
3. **Write** - The preference is written to `~/.config/opencode/opencode.json` and the list refreshes.

### Keybinds

| Key | Action |
|-----|--------|
| `enter` | Select target / confirm model |
| `/` | Filter the current list |
| `esc` | Back to targets / quit from targets |
| `q` | Back to targets / quit from targets |
| `ctrl+c` | Same as `q` |

## How it works

### Discovery

- **Built-in agents**: `build`, `plan` (primary), `general`, `explore` (subagent)
- **JSON agents**: From `agent.*` keys in `opencode.json` (excludes system agents: `compaction`, `title`, `summary`)
- **Markdown agents**: `~/.config/opencode/agents/*.md` and `.opencode/agents/*.md`
- **JSON commands**: From `command.*` keys in `opencode.json`
- **Markdown commands**: `~/.config/opencode/commands/*.md` and `.opencode/commands/*.md`
- **Models**: All entries under `provider.*.models` in `opencode.json`

### Config writes

Uses [tidwall/sjson](https://github.com/tidwall/sjson) for surgical JSON path updates that preserve existing formatting and comments. Sets `agent.<name>.model` or `command.<name>.model`. Clearing a preference deletes the key.

### Environment

| Variable | Purpose |
|----------|---------|
| `OPENCODE_CONFIG_DIR` | Override config directory (default: `~/.config/opencode`) |

## Development

```bash
go test ./...     # run tests
go vet ./...      # lint
make build        # build binary
make clean        # remove binary
```
