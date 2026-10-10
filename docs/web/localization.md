# Localization

Part of the [web client documentation](../web.md).

## Localization

The client speaks English and Russian. **No user-facing word appears anywhere
under `src/`**: every caption lives in `web/locales/en.json` and
`web/locales/ru.json`, reached by key. That is the whole design goal, and the
reason for most of what follows -- translating this app has to mean editing a
data file, never opening a component.

```tsx
const t = useT()
<Title order={2}>{t('login.title')}</Title>
<Text>{t('folders.newCharacterIn', { name })}</Text>
<Text>{t('choice.language', { count })}</Text>
```

**`i18next` + `react-i18next`, and only `src/lib/i18n/` may import them.**
`npm run lint:layers` enforces that, for the reason `@/ui` exists: a vendor
every layer may reach for is a vendor nothing can ever replace. Screens import
`useT` from `@/lib/i18n`, which also carries the key type -- `useT` is typed
from `en.json`, so a key the catalogue does not define will not compile.

**Fallback is per key, not per file.** A Russian catalogue that has translated
a button and not the paragraph beside it shows the button in Russian and the
paragraph in English. That partial state is what a growing locale actually
looks like, so it is the case that has to work well -- and it is the same rule
the Go catalogue applies to SRD prose, described in
[dnd.md](../dnd.md#localization). That is the runtime's rule; the gate is stricter
-- see [Keeping the catalogue honest](#keeping-the-catalogue-honest).

**Plurals are keys, not code.** Russian has four plural forms where English has
two, so `n === 1 ? 'x' : 'xs'` is wrong before the word order is. A plural is
`foo_one` / `foo_other` in the catalogue, called as `t('foo', { count })`, and
i18next picks between the forms with `Intl.PluralRules`. Russian adds `_few`
and `_many` in its own file and nothing in `src/` changes.

That is also why **counts are digits rather than words**: the build screen
reads "2 more languages", not "Two more languages". A Russian
numeral agrees in gender with the noun it counts -- два языка, две черты -- so a
shared table of spelled numbers is the same composition bug one level down.

**Nothing is glued together.** A label built as `` `${whose}Spell attack bonus` ``
is English word order written in TypeScript: a translator handed the fragments
cannot reorder them, because the code already did. Every such phrase is one
message with named arguments in it.

### What is not translated

- **`src/features/legal/attribution.ts` and the notices on `/legal`.** The SRD
  5.1 attribution is pinned to `data/pack/srd-5.1/ATTRIBUTION.md` by a test, and a
  translated licence notice is a different notice. See
  [licensing.md](../licensing.md).
- **`index.html`'s `<title>` and `<meta description>`, and the PWA manifest.**
  They are static, baked at build time, and stay English. This is not a
  preference: the browser and the OS read all three *before* the bundle is
  parsed -- they are what the install dialog, the home-screen label and the
  browser tab are drawn from -- so nothing in `src/` could swap them per
  locale whatever machinery existed there.
- **The product's name.** `easydnd.org` is a name, not a word: it is what
  people type, what the certificate says and what an issue is filed against.
  Translating it would produce a product nobody can find.

- **Language names in the switcher.** "English" and "Русский" are what each
  language calls itself, which is what somebody looking for one is looking for.

The static ones -- `index.html` and the manifest block in `vite.config.ts` --
carry an `i18n-exempt` comment naming the reason. Grep for it before extracting
strings: a marked string is one to leave exactly where it is.

### Choosing a language

`LocaleActions` sits in the header, left of the account and sign-out controls,
and is drawn for guests and signed-out visitors too -- somebody who cannot read
the landing page should not have to make an account before they can.

Each row in its menu carries a flag emoji beside the name (`LOCALE_FLAGS` in
`lib/i18n/locales.ts`). A language is not a country, so the flag is `aria-hidden`
decoration and the name it sits beside is what actually identifies the row.
Emoji rather than artwork: Windows ships no flag glyphs, so Chrome there draws
the two-letter code instead -- still the right pair of letters.

The choice is autodetected from the browser on first load and kept in
`sessionStorage` under `easydnd.locale`. **Not on the account**, so a guest has
one; **not in `localStorage`**, so it is this visit's business -- the same trade
`features/groups/inviteToken.ts` makes. The consequence is worth knowing rather
than discovering: a second tab autodetects again rather than inheriting the
choice.

Detection is fifteen hand-rolled lines in `lib/i18n/instance.ts` rather than
`i18next-browser-languagedetector`. That leaves `i18n.language` as the raw
tag, so `ru-RU` produces a locale of "ru-RU" that every consumer has to
normalise again, and it reads `sessionStorage` unguarded, so a private-mode
browser that refuses storage throws before the app renders.

The chosen language rides on **every** API request as `?locale=`, because the
server negotiates per request and a page cannot rewrite its own
`Accept-Language`. `lib/api/locale.ts` holds it; `src/test/setup.ts` resets it,
because the suite shares one module registry. The catalogue cache in
`lib/api/catalog.ts` is keyed by locale for the same reason -- without that, a
switch would go on serving English entries for the life of the tab.

### Keeping the catalogue honest

Two checks, both in `make web/lint`:

- **The compiler.** `useT` is typed from `en.json`, so a key that is not in the
  catalogue is a type error at the call site.
- **`npm run check:messages`.** The compiler cannot see the other direction --
  a key the catalogue defines that nothing renders any more -- and has nothing
  to say about Russian. The script fails on an unused key, on a Russian key
  with no English counterpart, and on **an English key Russian does not
  translate**.

  That last one is a failure rather than a printed percentage, although a
  partial locale is the normal state of a growing one. The per-key fallback is
  why: with a whole screen's keys missing it does exactly what it promises and
  serves English, so nothing looks broken enough to notice until somebody reads
  the screen. A fallback that good is what stops a gap surfacing on its own,
  which makes a number nobody reads the wrong instrument. Adding an English
  caption means adding its Russian in the same change.

  Keys are compared on their base name, so Russian keeps the `_few` and
  `_many` forms English has no use for and is never asked for a suffix its
  grammar does not want. The runtime fallback stays either way -- it still
  saves a user from a blank screen if a key slips through -- it just does not
  decide what "finished" means.

### The suite renders in English

Around six hundred assertions match visible copy --
`getByRole('button', { name: 'New group' })`. They hold with the captions in
`web/locales/` because `src/test/render.tsx` is the single seam: it wraps every render in a `LocaleProvider` pinned to English.
`renderAt(viewport, ui, 'ru')` is how a test asks for the other one.

The instance is built per render rather than shared, and that is not tidiness.
The suite runs with `isolate: false`, so a language set on a module-level
singleton by one file would be inherited by every file that ran after it, in
whatever order they happened to run -- which is the same hazard `vi.mock` is
banned for. A module of pure functions that needs words takes a `Translate` as
an argument instead; `features/character/settled.ts` and `options.ts` are the
worked examples, and `src/test/i18n.ts` is the translator their tests hand over.
