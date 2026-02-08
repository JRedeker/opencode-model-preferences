/**
 * Model Preferences Plugin for OpenCode
 *
 * Maps preferred models to agents, sub-agents, and slash commands
 * with immediate persistence to global config.
 */

import { tool } from "@opencode-ai/plugin";
import type { Plugin } from "@opencode-ai/plugin";
import { readGlobalConfig, writeModelPreference } from "./config.js";

// ---------------------------------------------------------------------------
// Local type definitions for SDK client response shapes.
// The SDK's OpencodeClient wraps all responses in { data, error, request,
// response }. We define a minimal PluginClient interface that mirrors this
// shape so we can unwrap .data safely.
// ---------------------------------------------------------------------------

interface Agent {
  name: string;
  mode: "primary" | "subagent" | "all";
  builtIn: boolean;
  model?: string;
}

interface Command {
  name: string;
  model?: string;
  agent?: string;
  builtIn?: boolean;
}

interface Provider {
  id: string;
  name: string;
  models: Record<string, { id: string; name: string }>;
}

/** Matches the SDK's RequestResult shape for the "fields" response style. */
interface SdkResponse<T> {
  data: T;
}

interface PluginClient {
  app: { agents: () => Promise<SdkResponse<Agent[]>> };
  command: { list: () => Promise<SdkResponse<Command[]>> };
  provider: {
    list: () => Promise<
      SdkResponse<{ all: Provider[]; connected: string[] }>
    >;
  };
}

interface ConfigEntry {
  model?: string;
}

/** System agents excluded from the listing — not user-configurable. */
const SYSTEM_AGENTS = new Set(["compaction", "title", "summary"]);

export const ModelPreferencesPlugin: Plugin = async (input) => {
  const client = input.client as unknown as PluginClient;
  const configDir = input.directory;

  return {
    tool: {
      model_prefs_list: tool({
        description:
          "List all agents, sub-agents, and custom slash commands with their current model preferences.",
        args: {},
        execute: async () => {
          const { data: agents } = await client.app.agents();
          const { data: commands } = await client.command.list();
          const config = readGlobalConfig(configDir);

          const userAgents = agents.filter(
            (a) => !SYSTEM_AGENTS.has(a.name),
          );
          const primaryAgents = userAgents.filter(
            (a) => a.mode === "primary" || a.mode === "all",
          );
          const subAgents = userAgents.filter(
            (a) => a.mode === "subagent",
          );
          const customCommands = commands.filter((c) => !c.builtIn);

          const agentConfig = (config.agent ?? {}) as Record<string, ConfigEntry>;
          const commandConfig = (config.command ?? {}) as Record<string, ConfigEntry>;

          const lines: string[] = [];

          lines.push("## Agents");
          lines.push("| Name | Model |");
          lines.push("|------|-------|");
          for (const a of primaryAgents) {
            const model = agentConfig[a.name]?.model ?? "none";
            lines.push(`| ${a.name} | ${model} |`);
          }

          lines.push("");
          lines.push("## Sub-Agents");
          lines.push("| Name | Model |");
          lines.push("|------|-------|");
          for (const a of subAgents) {
            const model = agentConfig[a.name]?.model ?? "none";
            lines.push(`| ${a.name} | ${model} |`);
          }

          lines.push("");
          lines.push("## Slash-Commands");
          lines.push("| Name | Model |");
          lines.push("|------|-------|");
          for (const c of customCommands) {
            const model = commandConfig[c.name]?.model ?? "none";
            lines.push(`| ${c.name} | ${model} |`);
          }

          return lines.join("\n");
        },
      }),

      model_prefs_set: tool({
        description:
          "Set or clear a model preference for an agent or slash command. Use model='none' to clear.",
        args: {
          type: tool.schema.enum(["agent", "command"]),
          name: tool.schema.string(),
          model: tool.schema.string(),
        },
        execute: async (args) => {
          const { type, name, model } = args;

          // Clear the preference when model is "none"
          if (model === "none") {
            writeModelPreference(configDir, [type, name, "model"], undefined);
            return `Successfully cleared model preference for ${type} '${name}'.`;
          }

          // Validate model exists in provider registry
          const { data: providerData } = await client.provider.list();
          const [providerId, ...modelParts] = model.split("/");
          const modelId = modelParts.join("/");

          const provider = providerData.all.find(
            (p) => p.id === providerId,
          );
          if (!provider || !provider.models[modelId]) {
            return `Error: Model '${model}' not found in provider registry. Check provider and model name.`;
          }

          // Write valid model to global config
          writeModelPreference(configDir, [type, name, "model"], model);
          return `Successfully set ${type} '${name}' model to '${model}'.`;
        },
      }),
    },
  };
};
