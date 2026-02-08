# opencode-model-preferences

An OpenCode plugin that lets you map preferred LLM models to specific agents, sub-agents, and slash commands. Preferences are persisted to your global OpenCode config and take effect on the next session.

## Installation

Add the plugin to your global OpenCode config at `~/.config/opencode/opencode.json`:

```jsonc
{
  "plugin": [
    "@goost/opencode-model-preferences"
    // ... other plugins
  ]
}
```

Or for local development, use the absolute path:

```jsonc
{
  "plugin": [
    "/path/to/opencode-model-preferences"
  ]
}
```

## Usage

### Interactive dialog

Run the `/model-preferences` slash command in any OpenCode session:

```
/model-preferences
```

This starts a guided dialog that:

1. Lists all agents, sub-agents, and custom commands with current model mappings
2. Lets you select a target to configure
3. Shows available models from connected providers
4. Applies your selection immediately to global config
5. Loops until you're done

### Direct tool usage

The plugin registers two tools that the AI can call directly:

**`model_prefs_list`** — Lists all configurable targets with their current model mapping.

**`model_prefs_set`** — Sets or clears a model preference.

Arguments:
| Argument | Type | Description |
|----------|------|-------------|
| `type` | `"agent"` or `"command"` | Whether the target is an agent or slash command |
| `name` | string | Name of the agent or command (e.g. `coder`, `deploy`) |
| `model` | string | Model ID in `provider/model-id` format, or `"none"` to clear |

Example: set the `coder` agent to use Claude Opus 4:
```
model_prefs_set type=agent name=coder model=anthropic/claude-opus-4-20250514
```

## How it works

Model preferences are stored in your global OpenCode config (`~/.config/opencode/opencode.json`) using the native config format:

```jsonc
{
  // Your existing config is preserved, including comments
  "agent": {
    "coder": {
      "model": "anthropic/claude-opus-4-20250514"
    }
  },
  "command": {
    "deploy": {
      "model": "openai/gpt-4o"
    }
  }
}
```

OpenCode's native model routing reads `agent.<name>.model` and `command.<name>.model` from config at session start. This plugin simply writes to those paths — no custom routing hooks needed.

## What gets listed

| Category | Included | Excluded |
|----------|----------|----------|
| **Agents** | All user-facing agents (coder, plan, ask, etc.) | System agents: `compaction`, `title`, `summary` |
| **Sub-Agents** | All sub-agents (explore, general, etc.) | System agents |
| **Slash-Commands** | Custom/loaded commands | Built-in commands (help, compact, etc.) |

## Invalid model errors

When you set a model that doesn't exist in your provider registry, the `model_prefs_set` tool rejects the change immediately with an error message. The config is not modified.

If a previously valid model becomes unavailable (e.g., the provider removes it), OpenCode's native `ModelNotFoundError` will block execution at session start with:

- The invalid model name
- Fuzzy suggestions for similar valid models

**To fix:** Run `/model-preferences` and set a valid model, or manually edit `~/.config/opencode/opencode.json` to remove or correct the mapping.

## Development

```bash
pnpm install
pnpm test        # Run tests
pnpm build       # Build for distribution
pnpm typecheck   # Type check
```

## License

MIT
