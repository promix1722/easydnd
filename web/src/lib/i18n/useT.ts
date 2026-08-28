import { useTranslation } from 'react-i18next'

import en from '@locales/en.json'

/**
 * The translator.
 *
 * A thin wrapper over react-i18next's `useTranslation`, and thin on purpose:
 * what it adds is the key type. `t` will not compile against a key that is not
 * in `web/locales/en.json`, so a caption cannot be referenced before it has
 * been written down, and a key cannot be renamed in the catalogue without the
 * compiler naming every screen that used it.
 *
 * That covers one direction. The other -- a key in the catalogue that nothing
 * renders any more -- is not something a type can see, and is what
 * `npm run check:messages` is for.
 *
 * ```tsx
 * const t = useT()
 * <Title>{t('auth.login.title')}</Title>
 * <Text>{t('folders.newCharacterIn', { name })}</Text>
 * <Text>{t('groups.memberCount', { count })}</Text>
 * ```
 */
export function useT(): Translate {
  const { t } = useTranslation()
  return t as Translate
}

/**
 * The translator, as a type.
 *
 * Exported because not everything that needs words is a component. A module
 * of pure functions cannot call a hook, so it takes one of these as an
 * argument -- which is the same answer this codebase gives everywhere else a
 * dependency has to be swappable: `InviteSheet` takes its `copyLink`, and
 * `features/character/labels.ts` takes its `t`.
 */
export type Translate = (key: MessageKey, options?: Record<string, unknown>) => string

/**
 * Every key the English catalogue defines.
 *
 * Derived from the JSON rather than declared beside it, so `web/locales/en.json`
 * stays the single source of truth and a translator's file is not shadowed by a
 * developer's list of what it is allowed to contain.
 *
 * Plural keys are stored with i18next's suffixes -- `foo_one`, `foo_other` --
 * and are called without one: `t('foo', { count })`. So the union strips a
 * trailing suffix and offers the base key alongside the literal ones. Russian
 * adds `_few` and `_many` in its own file, which never reach this type and do
 * not need to: they are forms of a key English has already declared.
 */
type Suffix = '_zero' | '_one' | '_two' | '_few' | '_many' | '_other'

type Base<K> = K extends `${infer Stem}${Suffix}` ? Stem : K

export type MessageKey = Base<keyof typeof en>
