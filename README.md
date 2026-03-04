# omp — OpenCode Model Preferences

A TUI for managing **model routing** in [OpenCode](https://github.com/anomalyco/opencode). Assign **roles** to agents/commands, then map each role to a model. When you apply routing, only roles with a model mapped actually write to `opencode.json` — unmapped roles leave agents unchanged.

## Roles

`omp` uses five named roles as the vocabulary for model assignment:

| Role | Purpose |
|------|---------|
| `brain` | Orchestration, planning, and reviewing |
| `tasker` | Purely agentic operations and tool calling |
| `builder` | Coding |
| `librarian` | Fact-checking and research |
| `fixer` | Expertise and problem solving |

Roles are purely for model mapping — they carry no context or system prompt.

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

1. **Agents view** (default) — Browse agents and commands. Each shows its current model and assigned role.
2. **Assign role** — Press `r` on an agent to assign one of the five roles (or clear it).
3. **Roles view** — Press `tab` to switch to the roles view. Each role shows its mapped model.
4. **Map model** — Press `enter` on a role to pick a model for it (or clear it).
5. **Apply** — Press `a` in the agents view to write models to `opencode.json`. Only targets with a role assigned AND whose role has a model mapped are affected.

### Keybinds

**Agents view:**

| Key | Action |
|-----|--------|
| `r` | Assign role to selected agent/command |
| `a` | Apply routing to opencode.json |
| `tab` | Switch to roles view |
| `/` | Filter the list |
| `q` / `ctrl+c` | Quit |

**Roles view:**

| Key | Action |
|-----|--------|
| `enter` | Set model for selected role |
| `tab` | Switch to agents view |
| `esc` | Back to agents view |
| `q` / `ctrl+c` | Quit |

### How routing works

When you press `a` to apply:
- For each target (agent/command), `omp` checks if it has a role assigned
- If yes, it looks up the model mapped to that role
- If the role has a model, it writes that model to `opencode.json`
- If the role has no model (unmapped), the target keeps its existing model
- Targets with no role assigned are not touched at all

> **Note:** Apply only writes to agents and commands that already have an entry in `opencode.json`. It does not create new agent entries.

## Routing config format

Routing is stored in `~/.config/opencode/omp-routing.json` (separate from `opencode.json`, which does not accept unknown keys):

```json
{
  "role_models": {
    "brain": "anthropic/claude-opus-4",
    "builder": "anthropic/claude-sonnet-4",
    "tasker": "openai/gpt-4o"
  },
  "target_roles": {
    "build": "brain",
    "plan": "brain",
    "general": "builder",
    "explore": "tasker"
  }
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
- **Markdown agents**: `~/.config/opencode/agents/*.md` and project `.opencode/agents/*.md` — `mode` from frontmatter determines classification
- **JSON agents**: From `agent.*` keys in `opencode.json` (excludes system agents: `compaction`, `title`, `summary`)
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
