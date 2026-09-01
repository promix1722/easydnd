import { createBrowserRouter } from 'react-router'

import { AccountScreen } from '@/features/account'
import { LoginScreen } from '@/features/auth'
import { BuildScreen, CharacterLogScreen, CharacterSheetScreen } from '@/features/character'
import { ImportCharacterScreen } from '@/features/characters'
import { DiceScreen } from '@/features/dice'
import { GameScreen, GamesScreen, SharedSheetScreen } from '@/features/games'
import { GroupListScreen, GroupScreen } from '@/features/groups'
import { LegalScreen } from '@/features/legal'
import { SpellScreen, SpellsScreen } from '@/features/spells'

import { LandingShell } from '@/shell/LandingShell'
import { RootGate } from '@/shell/RootGate'

import { HomeRoute } from './HomeRoute'
import { JoinRoute } from './JoinRoute'
import { NotFoundPage } from './NotFoundPage'
import { Private } from './Private'

/**
 * The complete route table -- one tree for both viewports and for both sides
 * of the sign-in boundary, the way internal/api/http/router.go is one file for
 * the whole API. Which chrome wraps it is RootGate's business; routes never
 * know the viewport, and never redirect on account of who is looking.
 *
 * Screens live in features/, not here: routes/ is the table, and a screen that
 * lives in it is drift. The exception is a page that belongs to the table
 * rather than to an aggregate -- NotFoundPage. `/characters/:id/build` is one
 * screen for creation and for level-up, because the API poses them as the same
 * question.
 *
 * nginx rewrites unknown paths to index.html, so a deep link lands here rather
 * than on the API's 404 envelope.
 */
export const router = createBrowserRouter([
  {
    path: '/',
    element: <RootGate />,
    children: [
      { index: true, element: <HomeRoute /> },

      // A character is somebody's, so these render the landing page to a
      // signed-out visitor rather than redirecting: the URL survives being
      // shared, and signing in fills it in.
      //
      // The same screen as `/characters/:id/build`, and not by coincidence:
      // creating is answering the first question, and a separate create page
      // was a second shape for one thing. Without an `:id` it holds the name
      // alone; answering it creates the character and replaces this URL with
      // the build one.
      {
        path: 'characters/new',
        element: (
          <Private>
            <BuildScreen />
          </Private>
        ),
      },
      // Ahead of characters/:id so the literal wins over the parameter.
      {
        path: 'characters/import',
        element: (
          <Private>
            <ImportCharacterScreen />
          </Private>
        ),
      },
      {
        path: 'characters/:id',
        element: (
          <Private>
            <CharacterSheetScreen />
          </Private>
        ),
      },
      {
        path: 'characters/:id/build',
        element: (
          <Private>
            <BuildScreen />
          </Private>
        ),
      },

      // The log rather than the sheet: same character, the record instead of
      // what the record means. Not a NAV_ITEMS entry -- it hangs off a
      // character, not off the app.
      {
        path: 'characters/:id/log',
        element: (
          <Private>
            <CharacterLogScreen />
          </Private>
        ),
      },

      // Groups. A group is several people's, so like a character these
      // branch to the landing page for a signed-out visitor rather than
      // redirecting -- which is what makes an invitation link survive being
      // opened by somebody who has not signed in yet.
      {
        path: 'groups',
        element: (
          <Private>
            <GroupListScreen />
          </Private>
        ),
      },
      // Ahead of groups/:id so the literal wins over the parameter. The
      // invitation token rides in the URL fragment, which the browser never
      // sends to any server -- see features/groups/inviteToken.ts.
      //
      // Not wrapped in Private, and that is the point: this is the one deep
      // link that routinely arrives at somebody with no account at all, so
      // the token has to be saved before the branch rather than inside the
      // screen that a signed-out visitor never reaches. JoinRoute does both.
      { path: 'groups/join', element: <JoinRoute /> },
      {
        path: 'groups/:id',
        element: (
          <Private>
            <GroupScreen />
          </Private>
        ),
      },

      // A shared character's sheet stays under its group, because sharing is a
      // group's doing and the group is what grants the read.
      {
        path: 'groups/:id/characters/:character',
        element: (
          <Private>
            <SharedSheetScreen />
          </Private>
        ),
      },

      // Games are their own section, so they sit at the top level rather than
      // under the group they are played at -- which is also what keeps
      // activeNavPath lighting Games instead of Groups when one is open.
      {
        path: 'games',
        element: (
          <Private>
            <GamesScreen />
          </Private>
        ),
      },
      {
        path: 'games/:id',
        element: (
          <Private>
            <GameScreen />
          </Private>
        ),
      },

      // The compendium's browsable half. Private like every section -- a
      // guest session passes -- though the data is nobody's; if a public
      // browser is ever wanted, the two catalog routes in the Go router are
      // the thing to move, not this.
      {
        path: 'spells',
        element: (
          <Private>
            <SpellsScreen />
          </Private>
        ),
      },
      {
        path: 'spells/:slug',
        element: (
          <Private>
            <SpellScreen />
          </Private>
        ),
      },

      // The way in. Public by necessity, and the one route that redirects on
      // account of who is looking -- a signed-in visitor is sent to the app,
      // because a login page inside the signed-in shell is nonsense.
      { path: 'login', element: <LoginScreen /> },

      // How this account is reached: its passkeys and its connected
      // providers. Private, because it is an inventory of somebody's ways in.
      {
        path: 'account',
        element: (
          <Private>
            <AccountScreen />
          </Private>
        ),
      },

      /*
       * A die, as a page.
       *
       * Deliberately absent from `ui/sections.ts`: it is not a section of the
       * app, it owns no other paths and it lights nothing in the desktop
       * navbar, which never offers it -- the die is a phone's, and
       * `shell/MobileShell.tsx` is the only chrome that links here. Private
       * only because the menu that reaches it is; the signed-out visitor has
       * their own die on the landing page.
       */
      {
        path: 'roll',
        element: (
          <Private>
            <DiceScreen />
          </Private>
        ),
      },

      { path: '*', element: <NotFoundPage /> },
    ],
  },

  // /legal sits outside RootGate: a licence notice you have to sign in to read
  // is not a notice. The SRD 5.1 data is CC-BY-4.0 and that licence expects its
  // attribution in the product, so the landing footer links here and this
  // renders for everybody. Absent from shell/nav.ts -- it is a document, not a
  // section of the app.
  {
    path: '/legal',
    element: <LandingShell />,
    children: [{ index: true, element: <LegalScreen /> }],
  },
])
