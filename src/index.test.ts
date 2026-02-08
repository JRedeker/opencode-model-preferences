import { describe, test, expect, beforeEach, afterEach } from "vitest";
import { ModelPreferencesPlugin } from "./index.js";
import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";

// ---------------------------------------------------------------------------
// Mock Types (matches OpenCode SDK shapes without importing it)
// ---------------------------------------------------------------------------

interface MockAgent {
  name: string;
  mode: "primary" | "subagent" | "all";
  builtIn: boolean;
  model?: string;
}

interface MockCommand {
  name: string;
  model?: string;
  agent?: string;
  builtIn?: boolean;
}

interface MockProvider {
  id: string;
  name: string;
  models: Record<string, { id: string; name: string }>;
}

/** Mirrors the SDK's RequestResult { data: T } wrapper. */
interface SdkResponse<T> {
  data: T;
}

interface MockPluginInput {
  client: {
    app: { agents: () => Promise<SdkResponse<MockAgent[]>> };
    command: { list: () => Promise<SdkResponse<MockCommand[]>> };
    provider: {
      list: () => Promise<
        SdkResponse<{
          all: MockProvider[];
          connected: string[];
        }>
      >;
    };
  };
  project: { name: string; path: string };
  directory: string;
  worktree: string;
  serverUrl: URL;
  $: unknown;
}

interface MockToolContext {
  sessionID: string;
  messageID: string;
  agent: string;
  abort: AbortSignal;
  metadata: () => void;
  ask: () => Promise<void>;
}

// ---------------------------------------------------------------------------
// Test Fixtures
// ---------------------------------------------------------------------------

const MOCK_AGENTS: MockAgent[] = [
  { name: "coder", mode: "primary", builtIn: true },
  { name: "plan", mode: "primary", builtIn: true },
  { name: "ask", mode: "primary", builtIn: true },
  { name: "explore", mode: "subagent", builtIn: true },
  { name: "general", mode: "subagent", builtIn: true },
  // System agents that should be excluded
  { name: "compaction", mode: "subagent", builtIn: true },
  { name: "title", mode: "subagent", builtIn: true },
  { name: "summary", mode: "subagent", builtIn: true },
];

const MOCK_COMMANDS: MockCommand[] = [
  { name: "deploy", builtIn: false },
  { name: "review", builtIn: false },
  // Built-in commands that should be excluded
  { name: "help", builtIn: true },
  { name: "compact", builtIn: true },
];

const MOCK_PROVIDERS: MockProvider[] = [
  {
    id: "anthropic",
    name: "Anthropic",
    models: {
      "claude-sonnet-4-20250514": {
        id: "claude-sonnet-4-20250514",
        name: "Claude Sonnet 4",
      },
      "claude-opus-4-20250514": {
        id: "claude-opus-4-20250514",
        name: "Claude Opus 4",
      },
    },
  },
  {
    id: "openai",
    name: "OpenAI",
    models: {
      "gpt-4o": { id: "gpt-4o", name: "GPT-4o" },
    },
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function createTempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), "model-prefs-test-"));
}

function cleanupTempDir(dir: string): void {
  fs.rmSync(dir, { recursive: true, force: true });
}

function createMockInput(configDir: string): MockPluginInput {
  return {
    client: {
      app: { agents: async () => ({ data: MOCK_AGENTS }) },
      command: { list: async () => ({ data: MOCK_COMMANDS }) },
      provider: {
        list: async () => ({
          data: {
            all: MOCK_PROVIDERS,
            connected: ["anthropic", "openai"],
          },
        }),
      },
    },
    project: { name: "test-project", path: configDir },
    directory: configDir,
    worktree: configDir,
    serverUrl: new URL("http://localhost:3000"),
    $: {},
  };
}

function createMockContext(): MockToolContext {
  return {
    sessionID: "test-session",
    messageID: "test-message",
    agent: "coder",
    abort: new AbortController().signal,
    metadata: () => {},
    ask: async () => {},
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("Model Preferences Plugin", () => {
  let tempDir: string;
  let configDir: string;

  beforeEach(() => {
    tempDir = createTempDir();
    configDir = tempDir;
  });

  afterEach(() => {
    cleanupTempDir(tempDir);
  });

  // =========================================================================
  // rq-kzNmbHfj: Plugin Registration
  // =========================================================================

  describe("Plugin Registration (rq-kzNmbHfj)", () => {
    test("registers exactly 2 tools: model_prefs_list and model_prefs_set", async () => {
      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);

      expect(hooks.tool).toBeDefined();
      const toolNames = Object.keys(hooks.tool!);
      expect(toolNames).toHaveLength(2);
      expect(toolNames).toContain("model_prefs_list");
      expect(toolNames).toContain("model_prefs_set");
    });

    test("each tool has description and args", async () => {
      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);

      for (const toolName of ["model_prefs_list", "model_prefs_set"]) {
        const t = hooks.tool![toolName];
        expect(t.description).toBeTruthy();
        expect(t.args).toBeDefined();
        expect(t.execute).toBeTypeOf("function");
      }
    });
  });

  // =========================================================================
  // rq-HERsbxz6: model_prefs_list Tool
  // =========================================================================

  describe("model_prefs_list Tool (rq-HERsbxz6)", () => {
    test("lists agents with no mappings set — shows 'none'", async () => {
      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_list.execute({}, ctx);

      // Should contain agent names
      expect(output).toContain("coder");
      expect(output).toContain("plan");
      expect(output).toContain("ask");
      // Should show 'none' for unmapped agents
      expect(output).toContain("none");
    });

    test("excludes system agents: compaction, title, summary", async () => {
      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_list.execute({}, ctx);

      expect(output).not.toContain("compaction");
      expect(output).not.toContain("title");
      expect(output).not.toContain("summary");
    });

    test("lists agents with existing mappings from config", async () => {
      // Write config with an existing mapping
      const configPath = path.join(configDir, "opencode.json");
      fs.writeFileSync(
        configPath,
        JSON.stringify({
          agent: {
            coder: { model: "anthropic/claude-sonnet-4-20250514" },
          },
        }),
      );

      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_list.execute({}, ctx);

      expect(output).toContain("anthropic/claude-sonnet-4-20250514");
    });

    test("segments output into Agents, Sub-Agents, and Slash-Commands", async () => {
      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_list.execute({}, ctx);

      expect(output).toContain("Agents");
      expect(output).toContain("Sub-Agents");
      expect(output).toContain("Slash-Commands");
    });

    test("lists custom commands but not built-in commands", async () => {
      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_list.execute({}, ctx);

      expect(output).toContain("deploy");
      expect(output).toContain("review");
      // Built-in commands like help should be excluded
      expect(output).not.toMatch(/\bhelp\b/);
      expect(output).not.toMatch(/\bcompact\b/);
    });

    test("categorizes agents by mode: primary → Agents, subagent → Sub-Agents", async () => {
      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_list.execute({}, ctx);

      // coder (primary) should be in Agents section, before Sub-Agents
      const agentsIdx = output.indexOf("Agents");
      const subAgentsIdx = output.indexOf("Sub-Agents");
      const coderIdx = output.indexOf("coder");
      const exploreIdx = output.indexOf("explore");

      expect(agentsIdx).toBeLessThan(subAgentsIdx);
      expect(coderIdx).toBeGreaterThan(agentsIdx);
      expect(coderIdx).toBeLessThan(subAgentsIdx);
      expect(exploreIdx).toBeGreaterThan(subAgentsIdx);
    });
  });

  // =========================================================================
  // rq-2AFh7KSa: model_prefs_set Tool
  // =========================================================================

  describe("model_prefs_set Tool (rq-2AFh7KSa)", () => {
    test("sets agent model preference and writes to config", async () => {
      const configPath = path.join(configDir, "opencode.json");
      fs.writeFileSync(configPath, "{}");

      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_set.execute(
        {
          type: "agent",
          name: "coder",
          model: "anthropic/claude-sonnet-4-20250514",
        },
        ctx,
      );

      // Should return success
      expect(output.toLowerCase()).toContain("success");
      // Verify config was written
      const config = JSON.parse(fs.readFileSync(configPath, "utf-8"));
      expect(config.agent?.coder?.model).toBe(
        "anthropic/claude-sonnet-4-20250514",
      );
    });

    test("sets command model preference", async () => {
      const configPath = path.join(configDir, "opencode.json");
      fs.writeFileSync(configPath, "{}");

      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_set.execute(
        { type: "command", name: "deploy", model: "openai/gpt-4o" },
        ctx,
      );

      expect(output.toLowerCase()).toContain("success");
      const config = JSON.parse(fs.readFileSync(configPath, "utf-8"));
      expect(config.command?.deploy?.model).toBe("openai/gpt-4o");
    });

    test("clears a model preference when model is 'none'", async () => {
      const configPath = path.join(configDir, "opencode.json");
      fs.writeFileSync(
        configPath,
        JSON.stringify({
          agent: { coder: { model: "anthropic/claude-sonnet-4-20250514" } },
        }),
      );

      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_set.execute(
        { type: "agent", name: "coder", model: "none" },
        ctx,
      );

      expect(output.toLowerCase()).toContain("clear");
      const config = JSON.parse(fs.readFileSync(configPath, "utf-8"));
      expect(config.agent?.coder?.model).toBeUndefined();
    });

    test("rejects invalid model that does not exist in provider registry", async () => {
      const configPath = path.join(configDir, "opencode.json");
      fs.writeFileSync(configPath, "{}");
      const originalContent = fs.readFileSync(configPath, "utf-8");

      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_set.execute(
        {
          type: "agent",
          name: "coder",
          model: "fake/nonexistent-model",
        },
        ctx,
      );

      // Should return error
      expect(output.toLowerCase()).toContain("error");
      expect(output).toContain("fake/nonexistent-model");
      // Config should not be modified
      const newContent = fs.readFileSync(configPath, "utf-8");
      expect(newContent).toBe(originalContent);
    });
  });

  // =========================================================================
  // rq-Qvl5PkuG: Global Config Persistence
  // =========================================================================

  describe("Global Config Persistence (rq-Qvl5PkuG)", () => {
    test("preserves JSONC comments when writing", async () => {
      const configPath = path.join(configDir, "opencode.json");
      const jsonc = `{
  // This is a comment
  "provider": {
    "anthropic": {
      "apiKey": "sk-test"
    }
  }
}`;
      fs.writeFileSync(configPath, jsonc);

      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      await hooks.tool!.model_prefs_set.execute(
        {
          type: "agent",
          name: "coder",
          model: "anthropic/claude-sonnet-4-20250514",
        },
        ctx,
      );

      const result = fs.readFileSync(configPath, "utf-8");
      expect(result).toContain("// This is a comment");
      expect(result).toContain("anthropic/claude-sonnet-4-20250514");
    });

    test("creates config file if it does not exist", async () => {
      const configPath = path.join(configDir, "opencode.json");
      // Don't create the file — it should not exist

      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      await hooks.tool!.model_prefs_set.execute(
        {
          type: "agent",
          name: "coder",
          model: "anthropic/claude-sonnet-4-20250514",
        },
        ctx,
      );

      expect(fs.existsSync(configPath)).toBe(true);
      const config = JSON.parse(fs.readFileSync(configPath, "utf-8"));
      expect(config.agent?.coder?.model).toBe(
        "anthropic/claude-sonnet-4-20250514",
      );
    });
  });

  // =========================================================================
  // rq-gXZ8NWRB: Agent Filtering
  // =========================================================================

  describe("Agent Filtering (rq-gXZ8NWRB)", () => {
    test("system agents compaction/title/summary are excluded", async () => {
      const input = createMockInput(configDir);
      const hooks = await ModelPreferencesPlugin(input as any);
      const ctx = createMockContext();

      const output = await hooks.tool!.model_prefs_list.execute({}, ctx);

      // User-facing agents present
      expect(output).toContain("coder");
      expect(output).toContain("plan");
      expect(output).toContain("explore");
      // System agents excluded
      expect(output).not.toContain("compaction");
      expect(output).not.toContain("title");
      expect(output).not.toContain("summary");
    });
  });
});
