import type { Prompt } from '@/lib/api'

import { offersOptions } from './options'

import { titleCase } from '@/domain'

/**
 * A choice named as a thing rather than asked as a question.
 *
 * "Two more languages", not "Choose 2 more languages". Both surfaces that show
 * a choice want the noun: the sheet lists what is left, and the build screen
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
 */
export function choiceName(prompt: Prompt): string {
  const { choose, kind } = prompt.choice
  switch (kind) {
    case 'race':
      return 'A race'
    case 'subrace':
      return 'A subrace'
    case 'background':
      return 'A background'
    case 'class':
      return 'A class'
    case 'subclass':
      return 'An archetype'
    case 'level':
      return 'Another level, in one of your classes'
    case 'alignment':
      return 'An alignment'
    case 'text':
      return 'A name'
    case 'proficiency':
      return prompt.heldOnly
        ? `${count(choose)} to double your proficiency in`
        : `${count(choose)} to be proficient in`
    case 'ability-bonus':
      return `${count(choose)} ability ${choose === 1 ? 'score' : 'scores'} to raise`
    // Two questions share this kind. The one that offers options is the
    // improvement a level grants; the one that offers none is the six numbers
    // a character starts with, which is a form rather than a choice of N.
    case 'ability-scores':
      return offersOptions(prompt)
        ? 'An improvement to your scores, or a feat'
        : `${count(choose)} ability scores`
    case 'language':
      return `${count(choose)} more ${choose === 1 ? 'language' : 'languages'}`
    case 'equipment':
      return 'Starting equipment'
    case 'personality':
      return `${count(choose)} personality ${choose === 1 ? 'trait' : 'traits'}`
    case 'ideal':
      return 'An ideal'
    case 'bond':
      return 'A bond'
    case 'flaw':
      return 'A flaw'
    case 'spell':
      return `${count(choose)} ${choose === 1 ? 'spell' : 'spells'}`
    default:
      return `${count(choose)} of ${titleCase(kind)}`
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
 * `undefined` for every other kind, which is what makes this the test as well
 * as the table.
 */
const WRITTEN: Record<string, { path: string; noun: string }> = {
  personality: { path: 'identity.personalityTraits', noun: 'Personality trait' },
  ideal: { path: 'identity.ideals', noun: 'Ideal' },
  bond: { path: 'identity.bonds', noun: 'Bond' },
  flaw: { path: 'identity.flaws', noun: 'Flaw' },
}

/** Where a written question's answer goes, or undefined if it is picked. */
export function writtenAs(prompt: Prompt): { path: string; noun: string } | undefined {
  return WRITTEN[prompt.choice.kind]
}

/**
 * What a written answer is called, read back from the path it settled.
 *
 * The same table from the other end, so that the block heading a decided trait
 * and the field that wrote it cannot come to call it two different things.
 */
export function writtenLabel(path: string): string | undefined {
  return Object.values(WRITTEN).find((each) => each.path === path)?.noun
}

/** Small counts read as words; large ones read as numbers. */
function count(n: number): string {
  return ['Zero', 'One', 'Two', 'Three', 'Four', 'Five', 'Six'][n] ?? String(n)
}
