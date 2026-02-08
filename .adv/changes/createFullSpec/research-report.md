# Architecture Research: createFullSpec

## Summary

Four architectural decisions were validated against OpenCode source code. Two major simplification opportunities were identified: native model routing eliminates all custom routing code, and native model validation eliminates the planned `src/validation.ts` module. The remaining implementation surface is a 2-tool persistence layer plus a slash command definition.

## Validated Decisions

| Decision | Status | Evidence |
|----------|--------|----------|
| Plugin + tool registration via `@opencode-ai/plugin` | VALIDATED | Matches SDK design, proven by Advance plugin (30 tools) |
| `agent.<name>.model` / `command.<name>.model` config fields | VALIDATED | Native routing in `prompt.ts`: `input.model → agent.model → lastModel()` |
| AI-orchestrated dialog via `question` tool | VALIDATED | Only viable pattern — `tui.openModels()` has no callback/context |
| Zero new runtime dependencies | VALIDATED | Only `jsonc-parser` needed (already in OpenCode's node_modules) |

## Simplification Opportunities

| Current Plan | Simpler Alternative | Effort Saved | Recommendation |
|--------------|---------------------|--------------|----------------|
| `src/validation.ts` with custom hooks | Drop entirely — native `ModelNotFoundError` blocks + suggests | ~100 LOC | **Remove from plan** |
| Custom model routing hooks | Drop entirely — config writes are the routing | ~80 LOC | **Remove from plan** |
| `src/dialog.ts` as separate module | Inline into tool execute — only ~20 lines of format logic | ~1 file | **Merge into index.ts** |
| `src/preferences.ts` as separate module | Merge into `src/config.ts` — same concern | ~1 file | **Merge** |

## Concerns

### 1. Global Config Write Has No SDK Method

`client.config.update()` writes to **project** config, not `~/.config/opencode/opencode.json`. The user chose global persistence.

**Solution**: Direct file I/O with `jsonc-parser` (same library OpenCode core uses internally). Replicates the exact `patchJsonc()` approach from `Config.updateGlobal()`.

**Risk**: After writing the global config, OpenCode may not hot-reload. `Config.updateGlobal()` calls `Instance.disposeAll()` + emits events, but plugins have no access to those internals.

**Mitigation**: The config is read fresh on each agent switch / command execution. Since preferences are consumed when the agent/command next runs (not immediately), a restart is unnecessary.

### 2. No Established Plugin Precedent for Config Writes

Zero existing plugins modify OpenCode config. This is uncharted territory.

**Mitigation**: The write mechanism (`jsonc-parser` modify + applyEdits) is battle-tested in OpenCode core. The risk is low.

## Anti-Patterns Detected

None.

## Over-Engineering Flags

| Flag | Detail | Action |
|------|--------|--------|
| `src/validation.ts` | Native `Provider.getModel()` already throws `ModelNotFoundError` with fuzzy suggestions. Building a separate validation layer duplicates framework behavior. | **Remove from affected code list** |
| Separate `src/dialog.ts` + `src/preferences.ts` modules | For 2 tools and ~150 LOC total, 6 source files is excessive. | **Consolidate to 2 files: `src/index.ts` (plugin entry) + `src/config.ts` (read/write)** |
| Custom model routing | OpenCode's `createUserMessage()` and `command()` in `prompt.ts` fully handle `agent.model` and `command.model`. Any hook-based routing is redundant. | **Remove routing code entirely** |

## Detailed Findings

### 1. Config Persistence Mechanism
**Current:** Write to `~/.config/opencode/opencode.json` via SDK
**Research:** `client.config.update()` targets project config only. No SDK method for global writes. OpenCode core uses `jsonc-parser` for JSONC-safe global patching.
**Simpler Option:** Use `client.config.update()` for project-level storage (1 line). But user chose global — so use `jsonc-parser` file I/O.
**Recommendation:** Direct file I/O with `jsonc-parser` `modify()` + `applyEdits()`, mirroring `Config.updateGlobal()`.
**Sources:** `packages/opencode/src/config/config.ts` (updateGlobal, patchJsonc functions)

### 2. Dialog UX Pattern
**Current:** Custom dialog module with segmented lists
**Research:** Plugin tools return strings. AI model orchestrates UX. `question` tool is the native interactive primitive. `tui.openModels()` has no callback or context.
**Simpler Option:** This IS the simplest pattern. 2 data tools + AI orchestration via slash command instructions.
**Recommendation:** Build `model_prefs_list` (read) and `model_prefs_set` (write) tools. Let slash command template instruct AI to use `question` tool for selection flow.
**Sources:** `packages/opencode/src/question/index.ts`, `packages/opencode/src/tool/question.ts`

### 3. Model Routing
**Current:** Planned custom routing hooks
**Research:** Agent routing: `input.model → agent.model → lastModel()` in `prompt.ts`. Command routing: `command.model → agent.model → input.model → lastModel()`. Both fully native.
**Simpler Option:** Write config. Done. Zero routing code needed.
**Recommendation:** Remove all routing code from plan. Config writes ARE the routing implementation.
**Sources:** `packages/opencode/src/session/prompt.ts` (createUserMessage, command functions)

### 4. Runtime Validation
**Current:** Planned `src/validation.ts` with hook-based validation
**Research:** `Provider.getModel()` throws `ModelNotFoundError` with fuzzy suggestions. Falls through to `UnknownError` (functional but generic).
**Simpler Option:** Trust native validation. Optionally add ~10 lines in `session.error` listener for enriched remediation message.
**Recommendation:** Drop `src/validation.ts`. Add optional error enrichment in plugin's event handler.
**Sources:** `packages/opencode/src/provider/provider.ts` (ModelNotFoundError, getModel)

## Action Items

- [x] Validate that config fields route natively (confirmed via source)
- [ ] Remove `src/validation.ts` from affected code list in proposal
- [ ] Remove `src/dialog.ts` and `src/preferences.ts` — consolidate into `src/index.ts` + `src/config.ts`
- [ ] Add `jsonc-parser` as dev dependency for global config JSONC-safe writes
- [ ] Add optional `session.error` enrichment for invalid-model remediation message
- [ ] Update proposal constraints to note that `jsonc-parser` is the one dependency (it's already in OpenCode's dep tree)

## Confidence

- **High**: Model routing is fully native (source code verified)
- **High**: Dialog pattern is validated (Advance plugin proves 30-tool pattern works)
- **High**: Native validation exists and blocks on invalid models
- **Medium**: Global config write approach (no SDK method, but mirrors OpenCode core approach)
- **Low**: Hot-reload behavior after global config write (may need restart for some edge cases)
