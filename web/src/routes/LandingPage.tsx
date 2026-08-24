import { Center, DragonMark } from '@/ui'

/**
 * What a signed-out visitor sees at `/`: the mark, and nothing else.
 *
 * There is no copy here on purpose. The header above it already carries the
 * name and the one control that matters -- `shell/SignInActions.tsx`'s "Log
 * in" -- and everything the old pitch claimed about rules and level-up is
 * either invisible until you are inside or better said on `/login`, which is
 * one press away and has the room to say what each way in costs.
 *
 * The passkey warning went with the pitch rather than being kept here.
 * `features/auth/LoginScreen.tsx` renders the same "this browser cannot use
 * passkeys" alert, and it does so on the page where the guest button that
 * still works actually lives, so a duplicate here would only be a sentence a
 * visitor reads twice on the way to the same place.
 *
 * The mark is sized against the viewport rather than to a fixed number: it is
 * a hero on a desktop and merely large on a phone, without a viewport branch.
 *
 * With nothing else on the page, the mark sits in the middle of the window --
 * the middle of the window itself, not of the space left under the header, so
 * it reads as centred rather than as nudged down by the chrome above it.
 *
 * `Center` centres what it is given inside its own box, and that box is
 * content-height by default, so it has to be given a height to centre within.
 * The box starts below the header, at `header + padding` down the viewport, so
 * for its middle to land on `50dvh` its height is the viewport less *twice*
 * that offset. It therefore stops short of the bottom of the page, which is
 * the point: the space it leaves at the bottom is what the header takes at the
 * top. Both terms come from `AppShell`'s own custom properties rather than
 * from repeating `56` and `lg` here, so changing the header height or the
 * shell padding in `shell/LandingShell.tsx` moves the mark with it.
 *
 * `min-height` and not `height`, so a viewport too short for the mark grows
 * and scrolls rather than clipping it.
 */
export function LandingPage() {
  return (
    <Center
      mih="calc(100dvh - (var(--app-shell-header-offset, 0rem) + var(--app-shell-padding, 0rem)) * 2)"
      py="xl"
    >
      <DragonMark size="min(64vw, 300px)" />
    </Center>
  )
}
