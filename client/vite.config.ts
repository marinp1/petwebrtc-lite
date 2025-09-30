import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const { VITE_PROXY_TARGET } = loadEnv(mode, "./");
  return {
    base: "./",
    build: {},
    dev: {},
    server: {
      proxy: {
        "^/camera*": {
          target: VITE_PROXY_TARGET,
          changeOrigin: true,
          secure: false,
        },
      },
    },
  };
});
