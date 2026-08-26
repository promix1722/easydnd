/**
 * What a page's body is doing, and the adapter from a resource to it.
 *
 * A union rather than three loose booleans, for the same reason `useResource`
 * models its outcome as one: "failed, with data still on screen" and "loading,
 * having already failed" are states nobody wants and every set of booleans can
 * express.
 *
 * It lives beside `Page` rather than inside it because `pageState` is a
 * function, and oxlint's `react/only-export-components` allows a component's
 * module to export a constant beside it but not a helper.
 */
export type PageState =
  | { kind: 'ready' }
  | { kind: 'loading'; what?: string }
  | { kind: 'failed'; title: string; detail: string; onRetry?: () => void }

/** The shape of `lib/useResource`'s return, named structurally so `ui/` need not import it. */
interface Loaded<T> {
  data: T | null
  error: string | null
  loading: boolean
}

/**
 * A resource, as a page state.
 *
 * `title` is what the failure alert is headed -- "Could not load your groups",
 * naming the thing rather than the verb. `fallback` is what to say when the
 * request failed without saying why, which the API envelope permits.
 *
 * Note that a resource which finished with `data` still null counts as failed.
 * That case is real -- a 200 with an empty envelope -- and a page that renders
 * its ready body against nothing is a page that throws.
 */
export function pageState<T>(
  resource: Loaded<T>,
  failed: { title: string; fallback: string; onRetry?: () => void },
): PageState {
  if (resource.loading) return { kind: 'loading' }
  if (resource.error !== null || resource.data === null) {
    return {
      kind: 'failed',
      title: failed.title,
      detail: resource.error ?? failed.fallback,
      ...(failed.onRetry ? { onRetry: failed.onRetry } : {}),
    }
  }
  return { kind: 'ready' }
}
