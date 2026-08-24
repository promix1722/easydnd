/**
 * The bundle's own version, injected at build time and also written to
 * dist/version.json by the versionManifest plugin in vite.config.ts.
 *
 * In `vite dev` the variable is unset and this reports "dev", matching what
 * the Go binary reports when its linker flag is absent. In a production build
 * it cannot be unset: the plugin fails the build first.
 */
export const WEB_VERSION: string = import.meta.env.VITE_APP_VERSION ?? 'dev'
