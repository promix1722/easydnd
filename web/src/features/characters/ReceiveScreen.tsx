import { useNavigate } from 'react-router'

import { classLine } from '@/domain'
import { clearInviteToken, COPY_LINK_STASH_KEY } from '@/features/groups'
import { acceptCopyLink, previewCopyLink } from '@/lib/api'
import { useT } from '@/lib/i18n'
import { useAction } from '@/lib/useAction'
import { useResource } from '@/lib/useResource'
import { Alert, Button, Group, Loader, Stack, Text, Title } from '@/ui'

const forget = () => clearInviteToken(COPY_LINK_STASH_KEY)

/**
 * The screen a signed-in visitor sees on a copy link: which character, and one
 * button that makes a copy of it theirs. The token arrives as a prop for the
 * reason JoinScreen's does -- see routes/ReceiveRoute.tsx.
 */
export function ReceiveScreen({ token }: { token: string }) {
  const t = useT()
  const navigate = useNavigate()
  const accept = useAction(acceptCopyLink)

  const { data, error, loading } = useResource(`copyLink:${token}`, (signal) => {
    if (token === '') return Promise.reject(new Error('no link'))
    return previewCopyLink(token, signal)
  })

  async function take() {
    const made = await accept.run(token)
    if (made === null) return
    forget()
    // Replacing the entry also scrubs the token out of the address bar.
    await navigate(`/characters/${made.id}`, { replace: true })
  }

  async function decline() {
    forget()
    await navigate('/characters')
  }

  if (token === '') {
    return (
      <Alert color="red" title={t('join.missing.title')}>
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{t('join.missing.detail')}</Text>
          <Button variant="light" onClick={() => void navigate('/characters')}>
            {t('receive.yourCharacters')}
          </Button>
        </Stack>
      </Alert>
    )
  }

  if (loading) {
    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">
          {t('receive.checking')}
        </Text>
      </Group>
    )
  }

  if (error !== null || data === null) {
    return (
      <Alert color="red" title={t('receive.unusable.title')}>
        <Stack gap="xs" align="flex-start">
          <Text size="sm">{error ?? t('join.unusable.detail')}</Text>
          <Button variant="light" onClick={() => void decline()}>
            {t('receive.yourCharacters')}
          </Button>
        </Stack>
      </Alert>
    )
  }

  return (
    <Stack gap="md">
      <div>
        <Title order={2}>{data.name || t('characters.thisCharacter')}</Title>
        <Text c="dimmed" size="sm">
          {classLine(data.classes)}
        </Text>
        <Text size="sm" mt="xs">{t('receive.offer')}</Text>
      </div>
      {accept.error !== null && (
        <Alert color="red" title={t('receive.failed')}>
          {accept.error}
        </Alert>
      )}
      <Group gap="xs">
        <Button loading={accept.pending} onClick={() => void take()}>
          {t('receive.action')}
        </Button>
        <Button variant="default" onClick={() => void decline()}>
          {t('join.notNow')}
        </Button>
      </Group>
    </Stack>
  )
}
