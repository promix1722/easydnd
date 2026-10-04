/**
 * The bundle's own version, injected at build time and also written to
 * dist/version.json by the versionManifest plugin in vite.config.ts.
 *
 * A release reports its tag and everything else reports a short commit SHA;
 * `deploy/release-version.sh` is where that is decided, and `make web/dev`
 * passes it too, so the dev server reports this commit rather than a word.
 *
 * "dev" is the fallback for a Vite that was handed nothing at all -- a bare
 * `npx vite`, and the test suite, which is why the suite can rely on it. It
 * cannot happen in a production build: the plugin fails the build first.
 *
 * @/lib/version disables release checks in Vite development mode, regardless
 * of this display version. The frontend and API may restart independently;
 * reloading a dev tab cannot change the version Vite received at startup.
 * Unversioned bundles are also excluded from release checks.
 */
export const WEB_VERSION: string = import.meta.env.VITE_APP_VERSION ?? 'dev'
