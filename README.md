# omp — OpenCode Model Preferences

A TUI for managing model routing in [OpenCode](https://github.com/anomalyco/opencode).

You can route targets in two ways:
- **Slot mapping**: assign target -> slot, then slot -> model
- **Direct mapping**: assign target -> model directly (overrides slot mapping)

When you apply, only targets with an effective mapping are written to `opencode.json`.

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

1. **Assignments view** (default) — Browse agents and commands. Each shows current model and active mapping.
2. **Assign slot** — Press `s` to assign a slot (or clear slot assignment).
3. **Set direct model** — Press `m` to assign a model directly to the selected target (or clear it).
4. **Slots view** — Press `tab` to switch to slots. Manage slot->model mappings there.
5. **Manage slots** — In Slots view: `enter` set slot model, `r` rename, `n` add, `x` remove.
6. **Apply** — Press `a` in Assignments view to write effective mappings to `opencode.json`.

### Keybinds

**Assignments view:**

| Key | Action |
|-----|--------|
| `s` | Assign slot to selected agent/command |
| `m` | Set direct model for selected agent/command |
| `a` | Apply mappings to opencode.json |
| `tab` | Switch to slots view |
| `/` | Filter the list |
| `q` / `ctrl+c` | Quit |

**Slots view:**

| Key | Action |
|-----|--------|
| `enter` | Set model for selected slot |
| `r` | Rename selected slot |
| `n` | Add a new slot |
| `x` / `delete` / `backspace` | Remove selected slot |
| `tab` | Switch to assignments view |
| `esc` | Back to assignments view |
| `q` / `ctrl+c` | Quit |

### How slot-based routing works

When you press `a` to apply:
- If target has a non-empty `target_models[target]`, that direct model is used
- Otherwise, if target has `target_slots[target]` and that slot has a model, slot model is used
- Otherwise, target is left unchanged

Precedence is always: **direct target model > slot model > unchanged**.

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
  },
  "target_models": {
    "general": "openai/gpt-5",
    "deploy": "anthropic/claude-sonnet-4"
  }
}
```

`target_models` is optional. If present, it overrides slot routing per target.

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
