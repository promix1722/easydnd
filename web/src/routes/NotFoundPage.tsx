import { Link } from 'react-router'

import { Anchor, Stack, Text, Title } from '@/ui'

export function NotFoundPage() {
  return (
    <Stack gap="sm">
      <Title order={2}>Not found</Title>
      <Text c="dimmed">This page does not exist.</Text>
      <Anchor component={Link} to="/">
        Back to your characters
      </Anchor>
    </Stack>
  )
}
