import type { Prompt } from '@/lib/api'
import type { MessageKey, Translate } from '@/lib/i18n'

import { offersOptions } from './options'

import { titleCase } from '@/domain'

/**
 * A choice named as a thing rather than asked as a question.
 *
 * "2 more languages", not "Choose 2 more languages". Both surfaces that show a
 * choice want the noun: the sheet lists what is left, and the build screen
 * heads each block with the choice it opens onto -- and a list of imperatives
 * reads as a list of orders whether or not you can press it.
 *
 * It is the only place in the client that names a prompt kind for a person,
 * which is what keeps the sheet and the build screen from growing two
 * vocabularies for one server response.
 *
 * A kind this client has not learned yet still gets a name. The server may
 * grow a kind before the browser does, and a choice you cannot see is worse
 * than a choice named plainly.
 *
 * # Why every branch is a whole message
 *
 * This file used to build its phrases out of parts: a count, then a noun, then
 * a plural `s` chosen by `choose === 1`. Both halves of that are English
 * grammar written in TypeScript, and neither survives translation.
 *
 * Russian has four plural forms where English has two -- one язык, два языка,
 * пять языков -- so the ternary is wrong before the word order is. And the
 * word order is wrong too: a translator handed the fragments "2" and
 * "languages" cannot put them in the order Russian needs, because the code has
 * already decided it. So each branch names one message and passes it a count,
 * and the whole phrase -- numeral, noun, agreement and all -- lives in
 * web/locales/*.json where somebody who speaks the language can write it.
 *
 * The counts also stopped being spelled out. "Two more languages" was better
 * English than "2 more languages", and it is not recoverable: a Russian
 * numeral agrees in gender with the noun it counts (два языка, две черты), so
 * a shared table of spelled numbers is the same composition bug one level
 * down. Digits are the honest version.
 */
export function choiceName(t: Translate, prompt: Prompt): string {
  const { choose, kind } = prompt.choice
  switch (kind) {
    case 'race':
      return t('choice.race')
    case 'subrace':
      return t('choice.subrace')
    case 'background':
      return t('choice.background')
    case 'class':
      return t('choice.class')
    case 'subclass':
      return t('choice.subclass')
    case 'level':
      return t('choice.level')
    case 'alignment':
      return t('choice.alignment')
    case 'text':
      return t('choice.text')
    case 'proficiency':
      return prompt.heldOnly
        ? t('choice.proficiencyDouble', { count: choose })
        : t('choice.proficiency', { count: choose })
    case 'ability-bonus':
      return t('choice.abilityBonus', { count: choose })
    // Two questions share this kind. The one that offers options is the
    // improvement a level grants; the one that offers none is the six numbers
    // a character starts with, which is a form rather than a choice of N.
    case 'ability-scores':
      return offersOptions(prompt)
        ? t('choice.improvement')
        : t('choice.abilityScores', { count: choose })
    case 'language':
      return t('choice.language', { count: choose })
    case 'equipment':
      return t('choice.equipment')
    case 'personality':
      return t('choice.personality', { count: choose })
    case 'ideal':
      return t('choice.ideal')
    case 'bond':
      return t('choice.bond')
    case 'flaw':
      return t('choice.flaw')
    case 'spell':
      return t('choice.spell', { count: choose })
    default:
      return t('choice.unknown', { count: choose, kind: titleCase(kind) })
  }
}

/**
 * The four questions a player answers in their own words, and where each lands.
 *
 * They are the character's *inputs*, like a name and an alignment: they settle
 * a value on the sheet rather than naming anything in the compendium, so the
 * answer is the change that settles it and something has to say which path.
 * That table lives here rather than beside the alignment's in `BuildScreen`
 * because these also need a noun -- the field has to be labelled with what one
 * of them is -- and one table with both is better than two that can disagree.
 *
 * `noun` is a message key rather than the word, so the label and the heading
 * go on agreeing in whatever language is on screen. `undefined` for every
 * other kind, which is what makes this the test as well as the table.
 */
const WRITTEN: Record<string, { path: string; noun: MessageKey }> = {
  personality: { path: 'identity.personalityTraits', noun: 'written.personalityTrait' },
  ideal: { path: 'identity.ideals', noun: 'written.ideal' },
  bond: { path: 'identity.bonds', noun: 'written.bond' },
  flaw: { path: 'identity.flaws', noun: 'written.flaw' },
}

/** Where a written question's answer goes, or undefined if it is picked. */
export function writtenAs(prompt: Prompt): { path: string; noun: MessageKey } | undefined {
  return WRITTEN[prompt.choice.kind]
}

/**
 * What a written answer is called, read back from the path it settled.
 *
 * The same table from the other end, so that the block heading a decided trait
 * and the field that wrote it cannot come to call it two different things.
 */
export function writtenLabel(path: string): MessageKey | undefined {
  return Object.values(WRITTEN).find((each) => each.path === path)?.noun
}
