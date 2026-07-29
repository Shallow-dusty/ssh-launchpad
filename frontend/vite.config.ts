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
    server: {
      port: 34115,
      strictPort: true
    },
    build: {
      target: "es2022",
      sourcemap: true
    }
  };
});
