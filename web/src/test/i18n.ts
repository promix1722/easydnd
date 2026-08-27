import { createI18n, type Translate } from '@/lib/i18n'

/**
 * A translator for the modules that take one but are not components.
 *
 * `settled.ts`, `options.ts` and `promptNames.ts` are pure functions handed a
 * `t`, so their tests have to hand them one too. This is a real i18next
 * instance rather than a stub returning its key, and deliberately: what those
 * tests are checking is the *phrase* -- "2 more languages", "1 more language"
 * -- which is plural selection and interpolation doing their job. A stub would
 * assert that the code asked for the right key and prove nothing about what a
 * person reads.
 *
 * Built once per module load rather than per call. It is immutable -- pinned
 * to English, never switched -- so it carries nothing from one test file to
 * the next, which is the property `isolate: false` makes everybody check.
 */
const instance = createI18n('en')

export const testT: Translate = instance.t.bind(instance) as Translate
