import { constants, cpSync } from "node:fs";
import path from "node:path";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const { VITE_PROXY_TARGET } = loadEnv(mode, "./");
  cpSync(
    path.resolve("./node_modules/@mediapipe/tasks-vision/wasm"),
    path.resolve("./public/wasm"),
    { recursive: true, mode: constants.COPYFILE_FICLONE },
  );
  return {
    base: "./",
    build: {},
    dev: {},
    server: {
      proxy: {
        "/cameras": {
          target: VITE_PROXY_TARGET,
          changeOrigin: true,
          secure: false,
        },
        "^/camera*": {
          target: VITE_PROXY_TARGET,
          changeOrigin: true,
          secure: false,
        },
      },
    },
  };
});
