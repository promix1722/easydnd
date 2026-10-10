import { useState } from 'react'

import { getVisibility, setVisibility } from '@/lib/api'
import { copyText } from '@/lib/clipboard'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import { Alert, Button, Group, ModalSheet, Stack, Switch, Text, TextInput } from '@/ui'

/**
 * The owner's switch for who may read a character: open it to anybody signed
 * in who has its link, or hide it again.
 *
 * One button in the sheet's header, opening a sheet that says what the switch
 * does before it is thrown -- "open" is the kind of word that can mean a
 * search engine, and here it means a link you hand to somebody. Open or
 * hidden, the groups a character is shared with still read it; this is the
 * way in for somebody who sits at none of them.
 *
 * The link is a field as well as a Copy button, because copying needs a secure
 * context and a dev box behind a plain HTTP proxy is not one: a link that can
 * be selected is still a link that can be sent.
 */
export function VisibilityAction({ id }: { id: string }) {
  const t = useT()
  const [opened, setOpened] = useState(false)
  const [copied, setCopied] = useState<boolean | null>(null)
  const state = useResource(`visibility:${id}`, (signal) => getVisibility(id, signal))
  const change = useAction(async (open: boolean) => {
    await setVisibility(id, open)
    state.reload()
  })
  const open = state.data?.public === true
  const link = `${window.location.origin}/shared/${encodeURIComponent(id)}`

  return <>
    <Button variant="light" onClick={() => { setCopied(null); setOpened(true) }}>{t('visibility.button')}</Button>
    <ModalSheet opened={opened} onClose={() => setOpened(false)} title={t('visibility.title')}>
      <Stack gap="md">
        {change.error !== null && <Alert color="red">{change.error}</Alert>}
        <Switch
          label={t('visibility.open')}
          description={t('visibility.hint')}
          checked={open}
          disabled={state.data === null || change.pending}
          onChange={(event) => void change.run(event.currentTarget.checked)}
        />
        {open && <>
          <TextInput readOnly aria-label={t('visibility.link')} value={link} onFocus={(event) => event.currentTarget.select()} />
          {copied === false && <Text size="sm" c="dimmed">{t('invite.clipboard.detail')}</Text>}
          <Group justify="flex-end">
            <Button variant={copied ? 'light' : 'filled'} onClick={() => void copyText(link).then(setCopied)}>
              {copied ? t('invite.copied') : t('invite.copyLink')}
            </Button>
          </Group>
        </>}
      </Stack>
    </ModalSheet>
  </>
}
