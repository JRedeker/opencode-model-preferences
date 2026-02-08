# Change: create full spec

## Why

Users need a first-class way to map preferred models to specific agents, sub-agents, and slash commands without manually editing config files. The current workflow is error-prone and does not provide an in-session UX for setting, validating, and persisting these preferences.

## What Changes

- Add a new OpenCode plugin that registers as both a plugin and a tool.
- Add a new in-session slash command: `/model-preferences`.
- Plugin exposes 2 tools: `model_prefs_list` (read current mappings) and `model_prefs_set` (write a mapping).
- AI orchestrates the UX flow using OpenCode's native `question` tool for row selection and model search/selection.
- Persist selections immediately to global OpenCode config (`~/.config/opencode/opencode.json`) using `jsonc-parser`.
- Model routing is 100% native — writing `agent.<name>.model` or `command.<name>.model` to config IS the routing. Zero custom hooks needed.
- Invalid model validation is native — `Provider.getModel()` throws `ModelNotFoundError` with fuzzy suggestions. No custom validation layer needed.

## Success Criteria

1. [ ] Running `/model-preferences` opens an interactive dialog that lists user-facing `Agents`, `Sub-Agents`, and loaded custom `Slash-Commands`, each with a model mapping column initialized to `none` unless explicitly set.
2. [ ] Selecting any row opens a searchable picker of currently registered `provider/model` values, and selecting a value persists immediately to `~/.config/opencode/opencode.json`.
3. [ ] On the next use of a mapped agent (for example `plan`), OpenCode resolves and switches to the mapped model automatically for that message.
4. [ ] If a mapped model no longer exists or is invalid, execution is blocked and returns a clear remediation message naming the target, invalid model, and exact fix path (`/model-preferences`).
5. [ ] Hidden/system agents (for example `compaction`, `title`, `summary`) are excluded from the dialog, while user-defined and built-in user-facing agents are included.

## Affected Code

- `package.json` — plugin metadata + `jsonc-parser` dependency
- `src/index.ts` — plugin entry point, 2 tools (`model_prefs_list`, `model_prefs_set`)
- `src/config.ts` — global config read/write using `jsonc-parser` `modify()` + `applyEdits()`
- `commands/model-preferences.md` — slash command template instructing AI to orchestrate the dialog
- `src/index.test.ts` — acceptance tests

## Research Findings (Post-Simplification)

| Original Plan | Simplified To | Reason |
|----------------|--------------|--------|
| `src/validation.ts` (~100 LOC) | Removed | Native `ModelNotFoundError` blocks + suggests via `Provider.getModel()` |
| `src/dialog.ts` | Merged into `src/index.ts` | Only ~20 lines of formatting logic for tool output |
| `src/preferences.ts` | Merged into `src/config.ts` | Same concern — config read/write |
| Custom model routing hooks (~80 LOC) | Removed | `prompt.ts` resolves `agent.model` / `command.model` natively |
| Zero dependencies | `jsonc-parser` (1 dep) | Required for global config writes — no SDK method for global config. Already in OpenCode's dep tree. |

See: `.adv/changes/createFullSpec/research-report.md` for full details.

## Constraints

- MUST: Store and read mappings from global config only (`~/.config/opencode/opencode.json`).
- MUST: Include only loaded custom slash commands in the slash-command section.
- MUST: Save mappings immediately after each selection (no explicit save step).
- MUST: Use `jsonc-parser` for JSONC-safe config writes (mirrors OpenCode core's `patchJsonc()` pattern).
- MUST NOT: Apply fallback models silently when a mapped model is invalid.
- SHOULD: Keep interaction parity with existing OpenCode model menu behavior (search-first, select-with-enter).

## Impact

- Affected specs: new capability (`model-preferences-routing`).
- Breaking changes: no.
- Dependencies: `jsonc-parser` (already in OpenCode's dependency tree).
