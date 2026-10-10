import { loginDevelopmentAccount, type DevelopmentAccount } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { useT } from '@/lib/i18n'

import { ActionIcon, Alert, Button, Card, Group, Menu, Stack, Text } from '@mantine/core'
import { IconUserCircle } from '@tabler/icons-react'

const accounts = ['master', 'player1', 'player2'] as const
const labels = { master: 'dev.master', player1: 'dev.player1', player2: 'dev.player2' } as const

/** The three seeded development identities, available on sign-in and in the header. */
export function DevAccounts({ compact = false, currentName }: { compact?: boolean; currentName?: string }) {
  const t = useT()
  const login = useAction(loginDevelopmentAccount)
  if (!import.meta.env.DEV) return null

  async function choose(account: DevelopmentAccount) {
    if (login.pending) return
    const result = await login.run(account)
    if (result === null) return
    const current = window.location.pathname
    const destination = result.game_ids.find((id) => current === `/games/${id}`) ?? result.game_ids[0]
    // A full navigation rebuilds auth and all resource/draft state for the new
    // session. The development cookie selector stays in this tab, so other
    // tabs keep their master/player identity.
    window.location.assign(destination ? `/games/${destination}` : '/games')
  }

  if (compact) {
    const label = currentName ? t('dev.switchAccount', { name: currentName }) : t('dev.accounts')
    return <Menu position="bottom-end" closeOnItemClick={false}>
      <Menu.Target>
        <ActionIcon variant="subtle" title={label} aria-label={label}><IconUserCircle size={20} /></ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Label>{label}</Menu.Label>
        {accounts.map((account) => <Menu.Item key={account} disabled={login.pending}
          onClick={() => void choose(account)}>{t(labels[account])}</Menu.Item>)}
        {login.error && <Menu.Label c="red">{login.error}</Menu.Label>}
      </Menu.Dropdown>
    </Menu>
  }
  return <Card withBorder>
    <Stack gap="sm">
      <Text fw={600}>{t('dev.accounts')}</Text>
      <Text size="sm" c="dimmed">{t('dev.accountsHint')}</Text>
      <Group gap="xs">
        {accounts.map((account) => <Button key={account} variant="light" disabled={login.pending}
          onClick={() => void choose(account)}>{t(labels[account])}</Button>)}
      </Group>
      {login.error && <Alert color="red">{login.error}</Alert>}
    </Stack>
  </Card>
}
