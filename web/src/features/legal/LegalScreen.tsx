import { useT } from '@/lib/i18n'
import { Anchor, Stack, Text, Title } from '@/ui'

import {
  CC_BY_URL,
  LICENSE_URL,
  PROJECT_COPYRIGHT,
  SRD_URL,
} from './attribution'

/**
 * The terms, in the product rather than only in the repository.
 *
 * CC-BY-4.0 covers the SRD 5.1 material this app is built on, and it expects
 * its notice where the work is used. `data/srd_5.1/ATTRIBUTION.md` has been
 * travelling in the release tarball to a directory nginx does not serve, which
 * `docs/licensing.md` recorded as an open gap; this page is what closes it.
 *
 * Public, and outside `RootGate` -- a licence notice you have to sign in to
 * read is not a notice. It wears `LandingShell` for everybody, the same
 * arrangement it has, and for this reason: it is not a section of
 * the app to navigate around, so it is absent from `shell/nav.ts` too.
 *
 * Two headings rather than one list, because the split is the whole point: the
 * code is this project's and MIT, the game material is Wizards of the Coast's
 * and CC-BY-4.0, and running them together would imply this project can license
 * the second.
 *
 * The attribution paragraph is reassembled here from the pieces in
 * `./attribution.ts` so the URLs can be links. The wording is not this file's
 * to change -- see that module's note on which copy is canonical.
 */
export function LegalScreen() {
  const t = useT()

  return (
    <Stack gap="lg" maw={640}>
      <Title order={2}>{t('legal.title')}</Title>

      {/*
        The two notices below stay in English, and deliberately.

        The SRD 5.1 paragraph is the attribution CC-BY-4.0 requires, pinned to
        `cmd/srdgen`'s `attribution` constant by `attribution.test.ts` -- see
        docs/licensing.md. A translated licence notice is a different notice,
        and this is the one place in the client where the exact words are the
        point rather than the meaning. The MIT paragraph beside it is left in
        English for the same reason and so the section does not read as half
        translated.
      */}
      <Stack gap="xs">
        <Title order={4}>easydnd</Title>
        <Text size="sm">
          {PROJECT_COPYRIGHT}. The service, this client, the SRD generator and the deploy
          scripts are released under the{' '}
          <Anchor href={LICENSE_URL} target="_blank" rel="noreferrer">
            MIT License
          </Anchor>
          . That covers the code and nothing else -- the game data below is not this
          project&apos;s to license.
        </Text>
      </Stack>

      <Stack gap="xs">
        <Title order={4}>SRD 5.1</Title>
        <Text size="sm">
          This work includes material taken from the System Reference Document 5.1 (&quot;SRD
          5.1&quot;) by Wizards of the Coast LLC, available at{' '}
          <Anchor href={SRD_URL} target="_blank" rel="noreferrer">
            {SRD_URL}
          </Anchor>
          . The SRD 5.1 is licensed under the Creative Commons Attribution 4.0 International
          License, available at{' '}
          <Anchor href={CC_BY_URL} target="_blank" rel="noreferrer">
            {CC_BY_URL}
          </Anchor>
          .
        </Text>
      </Stack>
    </Stack>
  )
}
