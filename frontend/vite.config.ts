import { defineConfig } from "vite";

export default defineConfig({
  clearScreen: false,
  server: {
    port: 34115,
    strictPort: true
  },
  build: {
    target: "es2022",
    sourcemap: true
  }
});
