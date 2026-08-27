import { Link } from 'react-router'

import { useT } from '@/lib/i18n'
import { Anchor, Stack, Text, Title } from '@/ui'

export function NotFoundPage() {
  const t = useT()

  return (
    <Stack gap="sm">
      <Title order={2}>{t('notFound.title')}</Title>
      <Text c="dimmed">{t('notFound.detail')}</Text>
      <Anchor component={Link} to="/">
        {t('notFound.back')}
      </Anchor>
    </Stack>
  )
}
