import { defineConfig } from "vite";
export default defineConfig(({ mode }) => ({
  build: { outDir: "web" },
  define: { __MODE__: JSON.stringify(mode), __EXAMPLE__: JSON.stringify(process.env.EXAMPLE) },
}));
