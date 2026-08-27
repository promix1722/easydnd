import { useEffect, useRef, useState } from 'react'

import type { InvitableRole } from '@/lib/api'
import { createInvite } from '@/lib/api'
import { copyText } from '@/lib/clipboard'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import {
  Alert,
  Button,
  Group,
  ModalSheet,
  Select,
  SHEET_COMBOBOX,
  Stack,
  Text,
  TextInput,
} from '@/ui'

export interface InviteSheetProps {
  groupId: string
  opened: boolean
  onClose: () => void
  /**
   * How the link reaches the clipboard. Defaults to the real thing.
   *
   * It is a prop so that a test can hand over a copy that fails, which is the
   * case this component exists for: outside a secure context there is no
   * clipboard, and the button must say so rather than quietly do nothing.
   * Mocking `@/lib/clipboard` instead would need the suite to isolate this file
   * from every other, and one optional argument is cheaper than a forked
   * worker. Nothing in the app passes it.
   */
  copyLink?: (text: string) => Promise<boolean>
}

/** The link the browser is looking at, with the token in the fragment. */
function linkFor(token: string): string {
  // A fragment, not a query string. The browser never sends it to any server,
  // so the token stays out of nginx's access log, out of the Referer header
  // and out of any link unfurler that fetches the URL.
  return `${window.location.origin}/groups/join#${token}`
}

/** How long the button admits to having copied something. */
const COPIED_FOR = 2000

/** Mints a shareable link and shows it once. */
export function InviteSheet({ groupId, opened, onClose, copyLink = copyText }: InviteSheetProps) {
  const t = useT()
  const [role, setRole] = useState<InvitableRole>('player')
  const [link, setLink] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [copyFailed, setCopyFailed] = useState(false)
  const field = useRef<HTMLInputElement>(null)
  const revert = useRef<number | null>(null)
  const invite = useAction(createInvite)

  useEffect(
    () => () => {
      if (revert.current !== null) window.clearTimeout(revert.current)
    },
    [],
  )

  async function mint() {
    const created = await invite.run(groupId, role)
    if (created === null) return
    setLink(linkFor(created.token))
  }

  async function copy() {
    if (link === null) return
    const ok = await copyLink(link)
    setCopied(ok)
    setCopyFailed(!ok)
    if (!ok) {
      // Nothing else can be done for them, so hand over the next best thing:
      // the link, selected, ready for the keyboard.
      field.current?.select()
      return
    }
    if (revert.current !== null) window.clearTimeout(revert.current)
    revert.current = window.setTimeout(() => setCopied(false), COPIED_FOR)
  }

  function close() {
    setLink(null)
    setCopied(false)
    setCopyFailed(false)
    invite.reset()
    onClose()
  }

  return (
    <ModalSheet opened={opened} onClose={close} title={t('invite.title')}>
      <Stack gap="sm">
        <Select
          label={t('invite.joinAs')}
          comboboxProps={SHEET_COMBOBOX}
          data={[
            { value: 'player', label: t('role.player') },
            { value: 'dm', label: t('role.dm') },
          ]}
          value={role}
          onChange={(value) => setRole((value as InvitableRole | null) ?? 'player')}
          allowDeselect={false}
        />

        {invite.error !== null && (
          <Alert color="red" title={t('invite.failed')}>
            {invite.error}
          </Alert>
        )}

        {link === null ? (
          <Group justify="flex-end">
            <Button variant="default" onClick={close}>
              {t('common.cancel')}
            </Button>
            <Button loading={invite.pending} onClick={() => void mint()}>
              {t('invite.createLink')}
            </Button>
          </Group>
        ) : (
          <Stack gap="xs">
            <TextInput
              ref={field}
              label={t('invite.linkLabel')}
              value={link}
              readOnly
              onFocus={(event) => event.currentTarget.select()}
            />
            {/* The only place the bargain is ever stated. A reusable link
                that cannot be withdrawn is a real trade, and the person
                sending it is the only one who can decide it is acceptable. */}
            <Text size="sm" c="dimmed">
              {t('invite.bargain')}
            </Text>
            {copyFailed && (
              // Copying needs a secure context, which a dev box behind a plain
              // HTTP proxy is not. Saying so beats a button that quietly does
              // nothing -- and the link is selected by now, so there is
              // something to actually do about it.
              <Alert color="yellow" title={t('invite.clipboard.title')}>
                {t('invite.clipboard.detail')}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant={copied ? 'light' : 'filled'} onClick={() => void copy()}>
                {copied ? t('invite.copied') : t('invite.copyLink')}
              </Button>
              <Button variant="default" onClick={close}>
                {t('common.done')}
              </Button>
            </Group>
          </Stack>
        )}
      </Stack>
    </ModalSheet>
  )
}
