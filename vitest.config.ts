import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
    include: ["src/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      include: ["src/**/*.ts"],
      exclude: ["src/**/*.test.ts", "src/__mocks__/**"],
    },
  },
  resolve: {
    alias: {
      "@opencode-ai/plugin": new URL(
        "./src/__mocks__/opencode-plugin.ts",
        import.meta.url
      ).pathname,
    },
  },
});
