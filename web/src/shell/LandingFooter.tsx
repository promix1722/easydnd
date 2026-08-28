import { Link } from 'react-router'

import { WEB_VERSION } from '@/lib/buildinfo'
import { useT } from '@/lib/i18n'
import { Anchor, Group, Text } from '@/ui'

/** The repository. Not written down anywhere else in the client. */
const REPO_URL = 'https://github.com/promix1722/easydnd'

/**
 * The one line along the bottom of the logged-out chrome: the source, the
 * terms, and which build this is.
 *
 * It exists for the middle one. The SRD 5.1 data this app is built on is
 * CC-BY-4.0, and that licence expects its notice in the *product* -- not only
 * in the repository, where `data/srd_5.1/ATTRIBUTION.md` has been carrying it
 * to a directory nginx does not even serve. `docs/licensing.md` recorded the
 * absence as an open gap; this footer and the `/legal` page it points at are
 * what close it. The GitHub link and the version are the two things that
 * belong in the same line once there is one.
 *
 * No `<nav>`, and that is load-bearing rather than an oversight.
 * `LandingShell.test.tsx` pins that the logged-out chrome exposes no navigation
 * landmark, because while nobody is signed in there is nowhere in the app to
 * navigate to -- and two links to static pages are not the app's navigation.
 * `AppShell.Footer` renders a `<footer>`, so this announces itself as
 * `contentinfo`, which is the landmark it actually is.
 *
 * The version is text rather than a link. Reading which build you are on is a
 * glance, and there is nowhere for it to lead: the page that used to compare it
 * against the API's was removed.
 *
 * It used to be truncated to seven characters, and that had to go with the
 * release identifier. Abbreviating was right when this was a forty-character
 * SHA that would have taken most of a 390px footer; against a tag it is
 * corruption -- `v10.20.30` would render as `v10.20.`, and a `-notest` release
 * would quietly stop saying so. The identifier is already short by
 * construction. See `deploy/release-version.sh`.
 */
export function LandingFooter() {
  const t = useT()

  return (
    <Group h="100%" px="md" gap="md" wrap="nowrap">
      <Anchor href={REPO_URL} target="_blank" rel="noreferrer" size="xs">
        GitHub
      </Anchor>
      <Anchor component={Link} to="/legal" size="xs">
        {t('legal.licences')}
      </Anchor>
      <Text size="xs" c="dimmed" ml="auto">
        {WEB_VERSION}
      </Text>
    </Group>
  )
}
