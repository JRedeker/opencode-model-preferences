# Model Preferences Routing

> **Version:** 1.0.0
> **Updated:** 2026-02-08

## Purpose

Capability: Model Preferences Routing

## Requirements

### Plugin Registration

**ID:** `rq-kzNmbHfj` | **Priority:** **[MUST]**

The plugin must export a valid OpenCode plugin via @opencode-ai/plugin that registers exactly 2 tools: model_prefs_list and model_prefs_set.

**Tags:** `plugin`, `registration`

#### Scenarios

**Plugin loads and registers tools** (`rq-kzNmbHfj.1`)

**Given:**
- the plugin is installed in opencode.json

**When:** OpenCode starts a session

**Then:**
- the plugin registers without errors
- tools model_prefs_list and model_prefs_set are available

---

### model_prefs_list Tool

**ID:** `rq-HERsbxz6` | **Priority:** **[MUST]**

The model_prefs_list tool must return a segmented table listing all user-facing Agents, Sub-Agents, and loaded custom Slash-Commands, each with their current model mapping. Mappings show 'none' if not explicitly set. Hidden/system agents (compaction, title, summary) must be excluded.

**Tags:** `tool`, `listing`, `agents`

#### Scenarios

**Lists agents with no mappings set** (`rq-HERsbxz6.1`)

**Given:**
- no model preferences are configured in global config
- the session has agents: coder, plan, ask

**When:** model_prefs_list is called

**Then:**
- output contains Agents section with coder, plan, ask
- each shows model as 'none'
- system agents compaction, title, summary are not listed

**Lists agents with existing mappings** (`rq-HERsbxz6.2`)

**Given:**
- global config has agent.coder.model set to 'anthropic/claude-sonnet-4-20250514'

**When:** model_prefs_list is called

**Then:**
- coder shows model as 'anthropic/claude-sonnet-4-20250514'
- other agents show 'none'

**Lists custom slash commands** (`rq-HERsbxz6.3`)

**Given:**
- custom commands 'deploy' and 'review' are loaded

**When:** model_prefs_list is called

**Then:**
- output contains Slash-Commands section with deploy and review
- built-in commands like help are not listed

**Segments output by type** (`rq-HERsbxz6.4`)

**Given:**
- agents, sub-agents, and commands all exist

**When:** model_prefs_list is called

**Then:**
- output is segmented into Agents, Sub-Agents, and Slash-Commands sections

---

### model_prefs_set Tool

**ID:** `rq-2AFh7KSa` | **Priority:** **[MUST]**

The model_prefs_set tool must accept a target type (agent or command), target name, and model ID in provider/model format. It must write the mapping to global config (~/.config/opencode/opencode.json) immediately. It must validate that the model exists in the provider registry before writing.

**Tags:** `tool`, `persistence`, `config`

#### Scenarios

**Sets agent model preference** (`rq-2AFh7KSa.1`)

**Given:**
- agent 'coder' exists
- model 'anthropic/claude-sonnet-4-20250514' is available

**When:** model_prefs_set is called with type='agent', name='coder', model='anthropic/claude-sonnet-4-20250514'

**Then:**
- global config is updated with agent.coder.model = 'anthropic/claude-sonnet-4-20250514'
- tool returns success confirmation

**Sets command model preference** (`rq-2AFh7KSa.2`)

**Given:**
- command 'deploy' exists
- model 'openai/gpt-4o' is available

**When:** model_prefs_set is called with type='command', name='deploy', model='openai/gpt-4o'

**Then:**
- global config is updated with command.deploy.model = 'openai/gpt-4o'
- tool returns success confirmation

**Clears a model preference** (`rq-2AFh7KSa.3`)

**Given:**
- agent 'coder' has a model mapping set

**When:** model_prefs_set is called with type='agent', name='coder', model='none'

**Then:**
- the agent.coder.model field is removed from global config
- tool returns confirmation that preference was cleared

**Rejects invalid model** (`rq-2AFh7KSa.4`)

**Given:**
- model 'fake/nonexistent-model' does not exist in any provider

**When:** model_prefs_set is called with model='fake/nonexistent-model'

**Then:**
- tool returns an error message
- global config is not modified
- error includes the invalid model name

---

### Global Config Persistence

**ID:** `rq-Qvl5PkuG` | **Priority:** **[MUST]**

All model preference mappings must be stored in and read from the global OpenCode config file (~/.config/opencode/opencode.json). Writes must use jsonc-parser to preserve JSONC formatting (comments, trailing commas). The plugin must never use client.config.update() which targets project config.

**Tags:** `persistence`, `config`, `global`

#### Scenarios

**Writes preserve JSONC formatting** (`rq-Qvl5PkuG.1`)

**Given:**
- global config contains comments and trailing commas

**When:** a model preference is set

**Then:**
- existing comments are preserved
- existing formatting is maintained
- the new field is added with consistent indentation

**Config file is created if missing** (`rq-Qvl5PkuG.2`)

**Given:**
- no global config file exists at ~/.config/opencode/opencode.json

**When:** a model preference is set

**Then:**
- the config file is created with proper JSON structure
- the mapping is written correctly

---

### Slash Command Dialog

**ID:** `rq-aMS7spq6` | **Priority:** **[MUST]**

The /model-preferences slash command must instruct the AI to orchestrate an interactive dialog using model_prefs_list for display, the question tool for selections, and model_prefs_set for persistence. The flow is: list all targets, let user select a row, show available models, let user pick, persist, and loop until user exits.

**Tags:** `command`, `ux`, `dialog`

#### Scenarios

**Command triggers model listing** (`rq-aMS7spq6.1`)

**Given:**
- the plugin is installed

**When:** user runs /model-preferences

**Then:**
- AI calls model_prefs_list
- AI presents the segmented list to the user

**User can select and set a model** (`rq-aMS7spq6.2`)

**Given:**
- the model preferences list is displayed

**When:** user selects an agent row and picks a model

**Then:**
- AI calls model_prefs_set with the selection
- AI confirms the mapping was saved
- AI offers to continue or exit

---

### No Silent Fallback on Invalid Model

**ID:** `rq-ldXrVKth` | **Priority:** **[MUST]**

When a mapped model is invalid or unavailable at runtime, OpenCode's native ModelNotFoundError must block execution. The plugin must not implement custom validation that silently falls back to a default model. The native error includes fuzzy suggestions for correction.

**Tags:** `validation`, `error-handling`

#### Scenarios

**Invalid model blocks agent execution** (`rq-ldXrVKth.1`)

**Given:**
- agent 'coder' is mapped to 'anthropic/deleted-model' which no longer exists

**When:** user invokes the coder agent

**Then:**
- execution is blocked
- error message includes the invalid model name
- error message includes fuzzy suggestions

---

### Agent Filtering

**ID:** `rq-gXZ8NWRB` | **Priority:** **[MUST]**

The model_prefs_list tool must exclude hidden/system agents from the listing. Specifically, agents named compaction, title, and summary must be filtered out. All other agents (built-in user-facing and user-defined) must be included and correctly categorized as Agent or Sub-Agent based on their mode field.

**Tags:** `filtering`, `agents`

#### Scenarios

**System agents excluded** (`rq-gXZ8NWRB.1`)

**Given:**
- session has agents: coder, plan, compaction, title, summary

**When:** model_prefs_list is called

**Then:**
- coder and plan are listed
- compaction, title, and summary are not listed

**Agents categorized by mode** (`rq-gXZ8NWRB.2`)

**Given:**
- agent 'coder' has mode 'primary'
- agent 'explore' has mode 'subagent'

**When:** model_prefs_list is called

**Then:**
- coder appears in Agents section
- explore appears in Sub-Agents section

---
