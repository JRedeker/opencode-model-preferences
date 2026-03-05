# omp — OpenCode Model Preferences

A TUI for managing **model slots** in [OpenCode](https://github.com/anomalyco/opencode). Create named **slots**, assign a model to each slot, then assign agents/commands to slots. When you apply, only slots with a model mapped actually write to `opencode.json` — unmapped slots leave agents unchanged.

## Slots

`omp` uses user-defined named slots for model assignment. By default, 4 slots are created (`Slot 1` through `Slot 4`), but you can rename them to match your workflow (e.g. "Brain", "Worker", "Fast", "Cheap").

Slots are purely for model mapping — they carry no context or system prompt.

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

1. **Assignments view** (default) — Browse agents and commands. Each shows its current model and assigned slot.
2. **Assign slot** — Press `s` on an agent to assign a slot (or clear it).
3. **Slots view** — Press `tab` to switch to the slots view. Each slot shows its mapped model.
4. **Map model** — Press `enter` on a slot to pick a model for it (or clear it).
5. **Apply** — Press `a` in the assignments view to write models to `opencode.json`. Only targets with a slot assigned AND whose slot has a model mapped are affected.

### Keybinds

**Assignments view:**

| Key | Action |
|-----|--------|
| `s` | Assign slot to selected agent/command |
| `a` | Apply slots to opencode.json |
| `tab` | Switch to slots view |
| `/` | Filter the list |
| `q` / `ctrl+c` | Quit |

**Slots view:**

| Key | Action |
|-----|--------|
| `enter` | Set model for selected slot |
| `tab` | Switch to assignments view |
| `esc` | Back to assignments view |
| `q` / `ctrl+c` | Quit |

### How slot-based routing works

When you press `a` to apply:
- For each target (agent/command), `omp` checks if it has a slot assigned
- If yes, it looks up the model mapped to that slot
- If the slot has a model, it writes that model to `opencode.json`
- If the slot has no model (unmapped), the target keeps its existing model
- Targets with no slot assigned are not touched at all

> **Note:** Apply only writes to agents and commands that already have an entry in `opencode.json`. It does not create new agent entries.

## Slots config format

Slot configuration is stored in `~/.config/opencode/omp-slots.json` (separate from `opencode.json`, which does not accept unknown keys):

```json
{
  "slots": [
    {"id": "slot-1", "name": "Brain", "model": "anthropic/claude-opus-4"},
    {"id": "slot-2", "name": "Worker", "model": "anthropic/claude-sonnet-4"},
    {"id": "slot-3", "name": "Fast", "model": "openai/gpt-4o"},
    {"id": "slot-4", "name": "Cheap", "model": ""}
  ],
  "target_slots": {
    "build": "slot-1",
    "plan": "slot-1",
    "general": "slot-2",
    "explore": "slot-3"
  }
}
```

### Migration from roles

If you previously used the role-based system (`omp-routing.json`), `omp` automatically migrates to slots on first launch:
- `brain` → Slot 1, `tasker` → Slot 2, `builder` → Slot 3, `librarian`/`fixer` → Slot 4
- The old `omp-routing.json` is renamed to `omp-routing.json.migrated`

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
