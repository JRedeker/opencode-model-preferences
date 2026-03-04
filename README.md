# omp — OpenCode Model Preferences

A TUI for managing **model routing** in [OpenCode](https://github.com/anomalyco/opencode). Instead of assigning a model to each agent individually, `omp` lets you define named **mappings** — each pairing an *orchestrator* model with a *worker* model — and activate one mapping to apply it globally.

## Role Definitions

`omp` uses five named roles as the vocabulary for model assignment. Each role represents a distinct capability profile:

| Role | Purpose |
|------|---------|
| `brain` | Orchestration, planning, and reviewing |
| `tasker` | Purely agentic operations and tool calling |
| `builder` | Coding |
| `librarian` | Fact-checking and research |
| `fixer` | Expertise and problem solving |

> **Note:** Roles are currently declarative vocabulary — they are not yet mapped to specific agents or commands. Mapping roles to targets is planned for a follow-on change.

## Why

OpenCode supports per-agent `model` overrides in `opencode.json`, but managing them individually across many agents is tedious. `omp` introduces a two-role routing model:

| Role | Applies to | Example agents |
|------|-----------|----------------|
| **Orchestrator** | Primary/all agents + commands | `build`, `plan`, custom primary agents |
| **Worker** | Subagents | `general`, `explore`, custom subagents |

You define named mappings (e.g. `fast`, `quality`) and activate one. `omp` writes the appropriate model to every agent in `opencode.json` in a single operation.

## Install

Requires Go 1.23+.

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

1. **Mapping list** — Browse your saved mappings. Each shows its orchestrator and worker model.
2. **Create/edit** — Press `enter` on a mapping to edit it, or select `(+ new mapping)` to create one. A form lets you set the name, orchestrator model, and worker model.
3. **Activate** — Press `a` on a mapping to apply it. `omp` writes the orchestrator model to all primary/all agents and commands, and the worker model to all subagents in `opencode.json`.
4. **Delete** — Press `d` to remove a mapping from the routing config.

### Keybinds

| Key | Action |
|-----|--------|
| `enter` | Edit selected mapping / confirm form |
| `a` | Activate selected mapping |
| `d` | Delete selected mapping |
| `/` | Filter the mapping list |
| `esc` / `q` | Back / quit |
| `ctrl+c` | Quit |

### Activation and existing per-target preferences

When you activate a mapping, `omp` overwrites any existing `agent.*.model` and `command.*.model` values in `opencode.json`. If any agents already have a model set, `omp` will warn you and require a second `a` press to confirm before applying.

> **Note:** Activation only writes to agents and commands that already have an entry in `opencode.json`. It does not create new agent entries.

## Routing config format

Mappings are stored in `~/.config/opencode/omp-routing.json` (separate from `opencode.json`, which does not accept unknown keys):

```json
{
  "mappings": [
    {
      "name": "fast",
      "orchestrator": "openai/gpt-4o-mini",
      "worker": "openai/gpt-4o-mini"
    },
    {
      "name": "quality",
      "orchestrator": "anthropic/claude-opus-4",
      "worker": "anthropic/claude-haiku-4"
    }
  ]
}
```

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
- **Markdown agents**: `~/.config/opencode/agents/*.md` and project `.opencode/agents/*.md` — `mode` from frontmatter determines role
- **JSON agents**: From `agent.*` keys in `opencode.json` (excludes system agents: `compaction`, `title`, `summary`)
- **Commands**: From `command.*` keys in `opencode.json` and markdown command files

### Role classification

| Agent mode | Role |
|-----------|------|
| `primary` | Orchestrator |
| `all` (default) | Orchestrator |
| `subagent` | Worker |
| Commands (any) | Orchestrator |

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
