import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    // "server-only" throws outside a React Server Component bundle; tests import those modules.
    alias: { "server-only": fileURLToPath(new URL("./src/test/empty.ts", import.meta.url)) },
  },
});
