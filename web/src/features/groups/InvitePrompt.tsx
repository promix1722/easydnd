import { Link, useLocation } from 'react-router'

import { Alert, Button, Stack, Text, Title } from '@/ui'

/**
 * What somebody sees when they follow an invitation link without being signed
 * in.
 *
 * Every other private route renders the bare landing page to a signed-out
 * visitor -- a mark, and the header's "Log in". That is right for a link to
 * somebody's character, which the visitor either recognises or does not. It is
 * wrong here: they were *sent* this link, they followed it deliberately, and a
 * dragon with no explanation does not tell them that the thing they came for
 * is waiting behind one button.
 *
 * It cannot name the group. Previewing an invitation needs a session, which is
 * the whole reason this screen exists, and opening that up so a stranger could
 * read a group's name off a link is not a trade worth making for one sentence.
 */
export function InvitePrompt({ hasToken }: { hasToken: boolean }) {
  const location = useLocation()

  if (!hasToken) {
    return (
      <Alert color="red" title="No invitation">
        That link is missing its invitation. Ask whoever sent it for a fresh one.
      </Alert>
    )
  }

  return (
    <Stack gap="md" align="flex-start">
      <div>
        <Title order={2}>You have been invited to a group</Title>
        <Text c="dimmed" size="sm">
          Sign in to join it. If you have never been here before, the same button makes you an
          account -- there is nothing to fill in.
        </Text>
      </div>
      {/* The location rides along exactly as the header's button does, so
          signing in comes back here. It carries the fragment with it, and the
          token is saved besides -- see inviteToken.ts, because Google's round
          trip drops the fragment on the floor. */}
      <Button component={Link} to="/login" state={{ from: location }}>
        Log in to join
      </Button>
    </Stack>
  )
}
