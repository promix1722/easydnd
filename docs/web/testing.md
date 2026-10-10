# Testing the web client

Part of the [web client documentation](../web.md).

## The test suite does not isolate test files

`vite.config.ts` runs the suite with `isolate: false`, so the test files a
worker picks up share **one** module registry and **one** DOM rather than
forking a fresh pair per file. Rebuilding the Mantine, embla and React module
graph 48 times over cost 36s of imports and 78s of DOM construction out of
229s total; sharing both took the run to 64s without a single assertion
changing. Passing `delay: null` to user-event (see `src/test/user.ts`) took it
to 38s from there, and the two rules below took it to 24s.

The DOM is **happy-dom**, not jsdom. At a hundred files the suite had grown to
186s of test time -- nearly all of it React rendering into the DOM -- which on
the three workers vitest forks on a four-core machine is 66s of wall. happy-dom
renders the same trees in half the time: 92s of test time, about 40s of wall.
It cost one test-side change and no production code: a storage spy targets
the `sessionStorage` instance rather than `Storage.prototype`, because
happy-dom's storage is a proxy a call never routes through the prototype of.
Neither engine computes layout
or evaluates `@media`, so everything below that says the DOM cannot measure
something is true of both.

Nothing sets the worker count. vitest's own default is the right answer on
every machine this runs on, and a number written down here would be wrong on
the next one.

What it costs is the guarantee that a test file starts from nothing, and two
things follow from that.

**`src/test/setup.ts` is what keeps the suite honest.** Its `afterEach`
unmounts the tree, resets the viewport, clears stubbed globals and empties the
catalogue request cache -- the only module-level mutable state in `src/`. If
you add another piece, reset it there in the same change. A test that passes
because of what ran before it is worse than one that fails. It is also the only
place those resets belong: a file that repeats one in its own hook is not safer,
only harder to read.

**`vi.mock` is not available, to anybody.** A shared registry means whichever
file loads a module first decides what every later file sees, so a mock
registered by the second file arrives too late. It does not fail loudly: the
test gets the real module, and the assertion breaks somewhere else, in whatever
order the files happened to run in.

**A component that needs a dependency swapped takes it as a prop.**
`InviteSheet` is the worked example. It must show an error when the clipboard
cannot be reached -- the bug it exists for -- so it accepts an optional
`copyLink` that defaults to the real `copyText`, and its test hands over a
`vi.fn()` that resolves `false`. No global is touched and nothing leaks into the
next file in the worker. `npm run lint:layers` fails on any `vi.mock` under
`src/`, and says this.

## Three rules about writing a test here

**A test runs at one viewport unless the tree branches on width.** The
components that call `useIsDesktop` do: in `ui/`, `DataList`, `ModalSheet`,
`TabDeck`, `ChoiceDetails` and -- for one style property, see below -- `TabRow`;
`RootShell`; and a handful of feature components (`Inventory`,
`SpellChoiceRow`, `SpellTags`, `GameTracker`, the landing carousel).

"Make the theme responsive" is the obvious wrong turn. Anything that must
differ by width and is not a *layout* belongs in `ui/app.css`, so nothing
branches in JavaScript and nothing re-renders at the breakpoint. It also means
no test here can assert a rendered size: the suite parses no CSS and the DOM
evaluates no `@media`.

`ui/Page` is deliberately **not** one of them, and its own test proves it
rather than asserting it in prose: the last case there compares the two
renderings byte for byte. The cheapest way to "fix" a future layout problem in
a shared page component would be to reach for `useIsDesktop`, and that test is
what goes red when somebody does. Nothing else can, because the suite runs
without CSS -- a `SimpleGrid cols={{ base: 2, sm: 3 }}` renders one DOM
whatever the width is -- so `describe.each(['mobile','desktop'])` around a tree
that reaches none of those components is the same assertion run twice against
byte-identical markup. `src/ui/TabRow.test.tsx` makes the same comparison: its
last test shows the two renderings equal but for the list's `flex-wrap`.

Where a block runs at one width, the comment above it names this rule, so the
next reader knows it was a decision. Where a block still runs at both -- the
group screens, `ModalSheet` itself, the rows of `CharacterListScreen`, and
`SheetBody`, whose deck draws a different tree at each width -- it is because
the swap is what the test is about.

The criterion is what the test *presses*, not what screen it is on.
`CharacterListScreen`'s row actions live inside `DataList`, so a test that
presses one belongs at both widths; the tests that press a folder's heading,
its action menu or **New folder** do not -- what they assert is a request body
or where a press navigated to, which is the same markup either way. They run at
one width. The folder dialogs are `ModalSheet`s and that swap is asserted on
its own terms in `src/ui/ModalSheet.test.tsx`, once, rather than several more
times here.

**Render the panel, not the page, when the panel is what is under test.**
`ProficienciesPanel.test.tsx`, `Vitals.test.tsx`, `SheetBody.test.tsx` and the
skills-panel block of `CharacterSheetScreen.test.tsx` mount their component
directly rather than the whole sheet, and keep one full-sheet test to hold the
seam. `SheetBody.test.tsx` is the newest of them and the clearest case: it takes
a projection and a compendium as props, so the phone's deck is tested without a
mocked fetch, a router or a single `findBy` -- and what is left in
`CharacterSheetScreen.test.tsx` is the seam, which is that the screen fetches
those two things and hands them on. A sheet test costs
about twice a panel test, because most of the sheet's weight is the eighteen
`ProficiencyMark`s -- each a Mantine `Tooltip`, each a floating-ui hook stack --
and mounting the page to read one panel pays for the other seventeen sections
too.

Where several read-only assertions share one fixture, one test that renders
once and asserts many times beats several that each re-mount it. Use
`expect.soft` when merging, so the merged test still reports every failure
instead of stopping at the first; a shared `beforeAll` render is not available,
because `src/test/setup.ts`'s global `afterEach` unmounts between tests and
exempting a file from that would give up the isolation rule above.

**Hand a component the state, rather than driving it there, when the driving
is not what is being tested.** `AbilityScoresForm.test.tsx` is the case: four
point-buy tests each spent two clicks and a Mantine `Combobox` dropdown moving
the method `Select` to a state the component takes as a prop, and one of them
clicked `Raise` twenty-one times to reach a spread it also takes as a prop.
`AbilityScoresForm` seeds its budget from `boughtFrom(method, scores)`, which is
the same value `change('point-buy')` sets, so the two are the same state. One
test still drives the `Select`, because that transition is what *it* is about;
the rest are handed `method="point-buy"`. The rule is not "avoid interactions"
-- it is that an interaction should be either the subject of the test or absent
from it.

The suite also does not process CSS (`css: true` is off). Nothing asserts on a
cascaded style -- the only style assertions read inline `element.style`, which
components and Mantine write from JS -- and the class names the tests query on
are emitted whether or not a stylesheet was ever parsed.
