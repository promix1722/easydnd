import type { createBrowserRouter } from 'react-router'
import type { PostHog, CaptureResult } from 'posthog-js'

import { WEB_VERSION } from './buildinfo'

type Router = ReturnType<typeof createBrowserRouter>
type User = { id: string; anonymous: boolean }
type Action = 'signed_in' | 'character_created' | 'group_joined' | 'game_created'
type AnalyticsConfig = { environment: string; token: string; host: string }

let client: Pick<PostHog, 'capture' | 'reset' | 'get_property' | 'get_distinct_id' | 'identify'> | undefined
let config: AnalyticsConfig
let starting: Promise<void> | undefined
let user: User | null = null
let identityKnown = false
let account: string | null = null
let route = '/'
let pageKey = ''
let sentPageKey = ''

const allowedProperties = new Set([
  'token', 'distinct_id', '$anon_distinct_id', '$device_id', '$user_id',
  '$session_id', '$window_id', '$is_identified', '$process_person_profile',
  '$lib', '$lib_version', '$browser', '$browser_version', '$os', '$os_version',
  '$device_type', '$screen_height', '$screen_width', '$viewport_height', '$viewport_width',
])

// Allowlist the fully enriched payload: SDK defaults also contain raw URLs and referrers.
function beforeSend(event: CaptureResult | null): CaptureResult | null {
  if (!event) return null
  delete event.$set
  delete event.$set_once
  delete event.$unset
  event.properties = Object.fromEntries(
    Object.entries(event.properties).filter(([key]) => allowedProperties.has(key)),
  )
  Object.assign(event.properties, {
    environment: config.environment,
    app_version: WEB_VERSION,
    account_type: user ? (user.anonymous ? 'guest' : 'registered') : 'visitor',
    $current_url: window.location.origin + route,
    $pathname: route,
  })
  return event
}

function syncIdentity() {
  if (!client || !identityKnown) return
  const next = user && !user.anonymous ? `${config.environment}:${user.id}` : null
  // Guest IDs are kept only in memory to detect account switches, never sent.
  const nextAccount = user?.id ?? null
  if ((account !== null && account !== nextAccount) ||
      (client.get_property('$user_id') && client.get_distinct_id() !== next)) {
    client.reset(true)
  }
  account = nextAccount
  if (next && client.get_distinct_id() !== next) client.identify(next)
}

function capturePage() {
  if (!client || !identityKnown || !pageKey || pageKey === sentPageKey) return
  sentPageKey = pageKey
  client.capture('$pageview')
}

export function setAnalyticsUser(next: User | null) {
  user = next
  identityKnown = true
  try {
    syncIdentity()
    capturePage()
  } catch { /* Analytics must never break authentication. */ }
}

export function track(action: Action) {
  try { client?.capture(action) } catch { /* Analytics is best effort. */ }
}

/** Called outside React, once. Configuration failure leaves analytics off until reload. */
export function startAnalytics(router: Pick<Router, 'state' | 'subscribe'>): Promise<void> {
  starting ??= initialize(router)
  return starting
}

async function initialize(router: Pick<Router, 'state' | 'subscribe'>) {
  try {
    const response = await fetch('/v1/analytics-config', { cache: 'no-store' })
    if (!response.ok) return
    config = await response.json() as AnalyticsConfig
    if (!config.token?.startsWith('phc_') || !config.host?.startsWith('https://') ||
        !['development', 'production'].includes(config.environment)) return
    const { default: posthog } = await import('posthog-js')
    const onNavigation = (state: Router['state']) => {
      if (!state.initialized || state.navigation.state !== 'idle') return
      route = '/' + state.matches.map(({ route: match }) => match.path)
        .filter(Boolean).join('/').split('/').filter(Boolean).join('/')
      // Query-only changes are filters/URL cleanup, not another page.
      pageKey = state.location.pathname
      try { capturePage() } catch { /* A blocked tracker cannot break navigation. */ }
    }
    onNavigation(router.state)
    posthog.init(config.token, {
      api_host: config.host,
      persistence: 'sessionStorage',
      persistence_name: `easydnd_${config.environment}`,
      autocapture: false,
      capture_pageview: false,
      capture_pageleave: false,
      capture_exceptions: false,
      capture_performance: false,
      capture_dead_clicks: false,
      enable_heatmaps: false,
      disable_session_recording: true,
      disable_surveys: true,
      disable_external_dependency_loading: true,
      advanced_disable_flags: true,
      save_referrer: false,
      save_campaign_params: false,
      person_profiles: 'identified_only',
      before_send: beforeSend,
      loaded: (loaded) => {
        client = loaded
        syncIdentity()
        capturePage()
        if (ssoSucceeded) finishAnalyticsRedirectSignIn(true)
      },
    })
    router.subscribe(onNavigation)
  } catch { /* Missing config, offline API, and blocked SDK all leave the app usable. */ }
}

const ssoKey = 'easydnd.analytics.sso'
let ssoSucceeded = false

export function beginAnalyticsRedirectSignIn() {
  try { sessionStorage.setItem(ssoKey, String(Date.now())) } catch { /* Storage is optional. */ }
}

export function finishAnalyticsRedirectSignIn(succeeded: boolean) {
  ssoSucceeded = succeeded
  if (succeeded && !client) return
  try {
    const started = Number(sessionStorage.getItem(ssoKey))
    sessionStorage.removeItem(ssoKey)
    if (succeeded && started > 0 && Date.now() - started < 10 * 60_000) track('signed_in')
  } catch { /* Storage is optional. */ }
}
