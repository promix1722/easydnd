import { Stack, Text, Title } from '@/ui'

/**
 * `/games`: a section of the app that is not built yet.
 *
 * It lives here rather than in `features/games/` on purpose, and for two
 * reasons. The first is the one `NotFoundPage` beside it already stands for:
 * a page belonging to the route table rather than to an aggregate belongs
 * with the table. The second is temporary -- the real game-session feature is
 * being written on another branch, and leaving `features/games/` unclaimed
 * means that work arrives as a new directory rather than as a conflict. When
 * it lands, the route swaps its element and this file is deleted.
 *
 * A visible section over a hidden route: `Run an adventure` is one of the
 * three things the landing page says this app is for, and the honest form of
 * an unbuilt third is a door that says so -- not a missing door, and not one
 * that opens onto a surface pretending to work. It is the same judgement that
 * keeps level-up off the build screen; see docs/web.md.
 */
export function GamesPlaceholder() {
  return (
    <Stack gap="sm">
      <div>
        <Title order={2}>Games</Title>
        <Text c="dimmed" size="sm">
          What a group actually plays.
        </Text>
      </div>
      <Text c="dimmed" size="sm">
        Nothing here yet -- running a game is not built. When it is, this is where a group's run of
        an adventure lives: the encounters, and what happened in them.
      </Text>
    </Stack>
  )
}
