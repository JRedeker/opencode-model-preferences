# OpenCode Plugin Model-Switching Capability Analysis

## Executive Summary

**Can an OpenCode plugin change the active session's model?**

**Answer: NO** - OpenCode plugins cannot directly change the active session model through:
- Command return directives
- Injected client calls  
- Hook mechanisms

However, **indirect approaches exist**:
1. Return structured data that guides LLM decision-making
2. Create custom commands that invoke model selection
3. Modify chat parameters/system prompts via hooks

---

## Plugin Architecture Overview

### Plugin Interface (Hooks)

OpenCode plugins implement the `Hooks` interface with these hook types:

```typescript
interface Hooks {
  // Define custom tools available to the LLM
  tool?: { [key: string]: ToolDefinition }
  
  // Custom authentication methods
  auth?: AuthHook
  
  // Hook into message processing (read-only)
  "chat.message"?: (input, output) => Promise<void>
  
  // Modify LLM parameters BEFORE sending
  "chat.params"?: (input, output: { 
    temperature, topP, topK, options 
  }) => Promise<void>
  
  // Modify LLM system prompt BEFORE sending
  "experimental.chat.system.transform"?: (
    input: { sessionID, model },
    output: { system: string[] }
  ) => Promise<void>
  
  // Intercept/modify tool execution
  "tool.execute.before"?: (input, output: { args }) => Promise<void>
  "tool.execute.after"?: (input, output: { title, output }) => Promise<void>
  
  // Lifecycle events
  event?: (input: { event: Event }) => Promise<void>
}
```

### Tool Return Type

Tools can **only return strings**:

```typescript
export type ToolDefinition = ReturnType<typeof tool>

tool({
  description: "...",
  args: { /* zod schema */ },
  async execute(args, context): Promise<string> {
    // ONLY string return allowed
    return "result text"
  }
})
```

### Plugin Client Access

Plugins receive a pre-configured SDK client:

```typescript
type PluginInput = {
  client: ReturnType<typeof createOpencodeClient>
  project: Project
  directory: string
  worktree: string
  serverUrl: URL
  $: BunShell
}
```

The `client` provides read/write access to:
- Sessions
- Messages
- Tools
- Files
- Logging
- App metadata

---

## Why Direct Model Switching Is Not Supported

### 1. **No Directive System**

OpenCode does **not** have a directive/instruction return type from plugins:

```typescript
// ❌ UNSUPPORTED - plugins cannot return directives
async execute(args): Promise<{ directive: "changeModel", model: "..." }> {
  // Plugin return type is: Promise<string>
  // Structured return objects are NOT supported
}
```

### 2. **Tool Outputs Are String-Only**

Tools convert all outputs to plain text before passing back to the LLM:

```typescript
// Plugin tool returns
return "success"

// LLM receives
Tool result: "success"
// LLM cannot interpret metadata or directives in the return
```

### 3. **No Session Update API in Client**

The `createOpencodeClient()` does not expose model-switching methods:

```typescript
// Available client methods (examples):
client.sessions.list()
client.sessions.show({ sessionID })
client.messages.list({ sessionID })
client.messages.show({ sessionID, messageID })
client.files.read({ filepath })
client.files.write({ filepath, content })
// ❌ NO: client.sessions.updateModel()
```

### 4. **Plugin Lifecycle is Per-Instance**

Plugins initialize once per OpenCode instance and cannot broadcast changes back to the session UI:

- Plugins are loaded during startup
- They receive session context as read-only input
- They cannot trigger session state mutations in the core

---

## Possible Workarounds (Limited)

### Option 1: Return Structured Data for LLM Consumption

A plugin can return **formatted text** that influences the LLM's next decision:

```typescript
async execute(args, context): Promise<string> {
  // Return structured text the LLM can parse
  return JSON.stringify({
    status: "success",
    recommendation: "model:anthropic/claude-sonnet",
    reason: "Switching to Sonnet for better reasoning"
  })
}
```

**Limitation**: The LLM must parse and decide to switch based on the returned text. Not guaranteed.

### Option 2: Create a Custom Command with Model Override

A command definition can specify a model:

```markdown
---
description: Switch to reasoning model
agent: general
model: anthropic/claude-opus  # Model is set in config
---

The user has requested heavy reasoning. 
Switch your context to use Claude Opus.
```

**When executed**: The custom command is processed by the specified model.

**Limitation**: Only affects that single command invocation; doesn't change the session's active model.

### Option 3: Modify System Prompt to Recommend Model Switch

Using the `"experimental.chat.system.transform"` hook:

```typescript
"experimental.chat.system.transform"?: (
  input: { sessionID, model },
  output: { system: string[] }
) => Promise<void>
```

A plugin can append system instructions:

```typescript
output.system.push(
  "If the task requires deep reasoning, recommend switching to a reasoning model."
)
```

**Limitation**: Still relies on the LLM to decide. No enforcement.

### Option 4: Use Built-in `/model` Command

The LLM can invoke the built-in `/model` command (when permitted):

```typescript
// Plugin can inform the LLM
return "To complete this task, use /model to switch to anthropic/claude-opus"
```

**Limitation**: 
- Requires user confirmation or agent autonomy
- The LLM must recognize and invoke the command
- Not a plugin-initiated change

---

## Architecture Diagram

```
┌─────────────────────────────────────┐
│     OpenCode Session (TUI)          │
│  - Active model (user-selected)     │
│  - Message history                  │
│  - Tool execution loop              │
└────────────┬────────────────────────┘
             │
             ├── Plugins (loaded at startup)
             │   ├── tool definitions
             │   ├── auth hooks
             │   ├── chat.params hook (⬅ can modify temp/topP)
             │   ├── chat.system hook (⬅ can append instructions)
             │   └── event listeners
             │
             ├── LLM Request
             │   ├── System prompt
             │   ├── Tool definitions
             │   └── Model (from session state)
             │
             └── LLM Response
                 ├── Text
                 ├── Tool calls
                 └── (NO model-switching directive)
```

---

## Real-World Examples from Codebase

### Advance Plugin (36 MCP tools)

The `advance` plugin returns **structured text** with JSON:

```typescript
// From gate.ts test
function extractJson(output: string): unknown {
  // Output starts with banner, JSON follows
  if (output.startsWith("╔")) {
    const jsonStart = output.indexOf("\n\n");
    return JSON.parse(output.slice(jsonStart + 2));
  }
  return JSON.parse(output);
}
```

**Mechanism**: Output formatted text → LLM parses → Makes decisions

### Morph Fast Apply Plugin

Returns plain text descriptions of changes:

```typescript
async execute(args, context): Promise<string> {
  // ... edit logic ...
  return `Edited file: ${target_filepath}\n\nLines: ${lineCount}`;
  // String-only return, no metadata
}
```

---

## What IS Possible with Plugins

✅ **DO implement**:
- Custom tools with arbitrary complexity
- Tool pre/post-execution hooks for logging
- LLM parameter tuning (temperature, topP, topK)
- System prompt augmentation
- Authentication/credential management
- Permission request interception
- Chat message processing (read-only)
- Session event subscriptions

❌ **CANNOT implement**:
- Direct model switching
- Session state mutations (outside plugin initialization)
- Directive-based control flow
- Returning structured data that changes behavior (only text)

---

## Recommendations

### If You Need Model Switching in a Plugin:

1. **Educate the LLM**: Use `"experimental.chat.system.transform"` to append instructions recommending when to switch models via the `/model` command.

2. **Provide Visibility**: Return clear text from your tool indicating what model would be optimal for the task.

3. **Let LLM Decide**: If the agent has autonomy, the LLM can invoke `/model <model-id>` based on task needs.

4. **Use Custom Commands**: For deterministic model switches, define custom commands with `model` specified in the frontmatter.

5. **Request a Feature**: Model-switching via plugin directive would require a core OpenCode change to:
   - Add a new hook type: `"session.updateModel"?`
   - Expose `client.sessions.updateModel(modelID, providerID)`
   - Handle UI/state propagation properly

---

## References

- **Plugin SDK**: `/home/jrede/dev/oc-plugins/morph-fast-apply/node_modules/@opencode-ai/plugin/dist/index.d.ts`
- **Hooks Interface**: Implements `chat.params`, `chat.system.transform`, `tool.execute.*` 
- **Tool Definition**: Returns `Promise<string>` only
- **Client API**: No `updateModel()` or session mutation methods
- **Real Examples**: 
  - Advance plugin (36 tools): `/home/jrede/dev/oc-plugins/advance/plugin/src/index.ts`
  - Morph plugin (1 tool): `/home/jrede/dev/oc-plugins/morph-fast-apply/index.ts`

