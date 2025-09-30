/** biome-ignore-all lint/correctness/noUnusedVariables: env files */
interface ImportMetaEnv {
  readonly VITE_PROXY_TARGET: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
