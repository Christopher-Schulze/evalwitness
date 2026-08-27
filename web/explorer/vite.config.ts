import path from "node:path";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  define: {
    "process.env.NODE_ENV": JSON.stringify("production"),
  },
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  build: {
    target: "es2022",
    sourcemap: false,
    lib: {
      entry: path.resolve(import.meta.dirname, "src/main.tsx"),
      name: "EvalWitnessEvidenceExplorer",
      formats: ["iife"],
      fileName: () => "explorer.js",
      cssFileName: "explorer",
    },
  },
});
