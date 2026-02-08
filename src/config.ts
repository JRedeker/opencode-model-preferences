/**
 * Global config read/write using jsonc-parser.
 *
 * Mirrors OpenCode core's patchJsonc() pattern for JSONC-safe
 * global config modification.
 */

import * as fs from "node:fs";
import * as path from "node:path";
import { modify, applyEdits, parse } from "jsonc-parser";

const CONFIG_FILENAME = "opencode.json";

function getConfigPath(configDir: string): string {
  return path.join(configDir, CONFIG_FILENAME);
}

/**
 * Read and parse the global OpenCode config file.
 * Returns an empty object if the file doesn't exist or can't be parsed.
 */
export function readGlobalConfig(configDir: string): Record<string, unknown> {
  const configPath = getConfigPath(configDir);
  try {
    const content = fs.readFileSync(configPath, "utf-8");
    return parse(content) ?? {};
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") {
      return {};
    }
    throw error;
  }
}

/**
 * Apply a single value at a JSON path, preserving JSONC comments and formatting.
 * Pass `undefined` as value to remove the key.
 */
function patchJsonc(
  input: string,
  jsonPath: (string | number)[],
  value: unknown,
): string {
  const edits = modify(input, jsonPath, value, {
    formattingOptions: { insertSpaces: true, tabSize: 2 },
  });
  return applyEdits(input, edits);
}

/**
 * Write a model preference to the global config.
 * Uses jsonc-parser to preserve comments and formatting.
 *
 * @param configDir - Directory containing opencode.json
 * @param jsonPath  - Path segments, e.g. ["agent", "coder", "model"]
 * @param value     - Model string to set, or undefined to remove the key
 */
export function writeModelPreference(
  configDir: string,
  jsonPath: string[],
  value: string | undefined,
): void {
  const configPath = getConfigPath(configDir);

  let content: string;
  try {
    content = fs.readFileSync(configPath, "utf-8");
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") {
      content = "{}";
    } else {
      throw error;
    }
  }

  const result = patchJsonc(content, jsonPath, value);
  fs.writeFileSync(configPath, result, "utf-8");
}
