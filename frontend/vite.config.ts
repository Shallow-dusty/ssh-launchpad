import { defineConfig, loadEnv } from "vite";
import packageMetadata from "./package.json";

export default defineConfig(({ mode }) => {
  const environment = loadEnv(mode, ".", "VITE_");
  const appVersion = environment.VITE_APP_VERSION || packageMetadata.version;
  return {
    clearScreen: false,
    define: {
      __APP_VERSION__: JSON.stringify(appVersion)
    },
    plugins: [{
      name: "preserve-dist-placeholder",
      generateBundle() {
        this.emitFile({ type: "asset", fileName: ".gitkeep", source: "" });
      }
    }],
    server: {
      port: 34115,
      strictPort: true,
      fs: {
        // Generated contracts/defaults are the only sibling build inputs.
        // Do not expose raw audit logs or the whole repository to the server.
        allow: [".", "../build/contracts"]
      }
    },
    build: {
      target: "es2022",
      sourcemap: true
    }
  };
});
