---
description: Configure model preferences for agents, sub-agents, and slash commands
---

You are managing model preferences for OpenCode. Follow this workflow precisely:

## Step 1: List current preferences

Call the `model_prefs_list` tool to get the current state of all agents, sub-agents, and custom slash commands with their model mappings.

Present the results to the user in the segmented table format returned by the tool.

## Step 2: Ask what to configure

Use the `question` tool to ask the user which target they want to configure. Present options from the list above (agents, sub-agents, and commands). Include a "Done" option to exit.

If the user selects "Done", thank them and end the session.

## Step 3: Show available models

Use the `question` tool to ask which model to assign. List the available models from connected providers. Include a "none" option to clear the current mapping.

## Step 4: Apply the preference

Call the `model_prefs_set` tool with the selected type (agent or command), name, and model.

Report the result to the user (success or error).

## Step 5: Loop

Go back to Step 1 to show the updated state. Continue until the user selects "Done".

## Important notes

- Models use the format `provider/model-id` (e.g. `anthropic/claude-sonnet-4-20250514`)
- Setting a model to `none` clears the preference (reverts to session default)
- Changes take effect on the next session — they are written to the global OpenCode config
- If a model is invalid, the tool returns an error. Help the user correct it.
