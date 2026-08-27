import { describe, expect, it } from 'vitest'

import type { Sheet } from '@/lib/api'
import { renderAt } from '@/test/render'

import { Vitals } from './Vitals'

/**
 * The row is built from four different corners of the projection, three of
 * which were being sent and drawn nowhere. What these pin is that each number
 * lands on its own card and that the row shrinks to what the character
 * actually has -- a rogue must not be given an empty spell slot to look at.
 */
const BASE: Sheet = {
  identity: { name: 'Zephyr', level: 1, experience: 0 },
  base: {
    hitPoints: { current: 9, max: 9 },
    deathSaves: { successes: 0, failures: 0 },
  },
  abilities: { scores: {}, modifiers: {} },
  skills: {},
  savingThrows: {},
  status: { armorClass: 15, initiative: 3, proficiencyBonus: 2, passivePerception: 13 },
  equipment: { equipped: [], backpack: [], loot: [] },
  resources: {},
  spells: {},
  actions: [],
}

/**
 * Each card's whole text -- label, value and hint run together.
 *
 * Read per card rather than by searching the page for a value, so a number
 * cannot pass a test by appearing under someone else's label.
 */
function cards(): string[] {
  return Array.from(document.querySelectorAll('.mantine-Card-root')).map(
    (card) => card.textContent ?? '',
  )
}

/** Just the labels, in document order -- the first line of each card. */
function labels(): string[] {
  return Array.from(document.querySelectorAll('.mantine-Card-root')).map(
    (card) => card.querySelector('.mantine-Text-root')?.textContent ?? '',
  )
}

function render(sheet: Sheet) {
  renderAt('desktop', <Vitals sheet={sheet} />)
}

describe('the vitals row', () => {
  it('draws passive Perception, which nothing on the sheet used to', () => {
    render(BASE)

    expect(cards()).toContain('Passive Perception13')
  })

  it('gives a caster three cards, because they answer three questions', () => {
    render({
      ...BASE,
      status: {
        ...BASE.status,
        spellcasting: [{ class: 'wizard', ability: 'int', saveDC: 12, attackBonus: 4 }],
      },
    })

    // The attack bonus is signed and the DC is not: one is added to a roll,
    // the other is a number to beat.
    expect(cards()).toContain('Spell attack bonus+4')
    expect(cards()).toContain('Spell save DC12')
    expect(cards()).toContain('Spellcasting abilityINT')
  })

  it('keeps the spell cards on a character who casts nothing', () => {
    render(BASE)

    // Every sheet is three lines of six, so a reader finds a number in the
    // same place on every character. "n/a" is the honest value: a rogue has
    // no spell save DC, which is not the same as having one nobody knows.
    expect(cards()).toContain('Spell attack bonusn/a')
    expect(cards()).toContain('Spell save DCn/a')
    expect(cards()).toContain('Spellcasting abilityn/a')
  })

  it('draws the same twelve cards whether or not the character casts', () => {
    render(BASE)
    const rogue = labels()

    render({
      ...BASE,
      status: {
        ...BASE.status,
        spellcasting: [{ class: 'wizard', ability: 'int', saveDC: 12, attackBonus: 4 }],
      },
    })
    // Two renders in one document, so the wizard's twelve are the last twelve.
    expect(labels().slice(rogue.length)).toEqual(rogue)
  })

  it('names each casting class when there is more than one', () => {
    // A cleric/wizard really does have two save DCs, and showing one of them
    // unlabelled would be showing the wrong one half the time.
    render({
      ...BASE,
      status: {
        ...BASE.status,
        spellcasting: [
          { class: 'cleric', ability: 'wis', saveDC: 13, attackBonus: 5 },
          { class: 'wizard', ability: 'int', saveDC: 12, attackBonus: 4 },
        ],
      },
    })

    // Lower-cased mid-phrase, which it was not before: the label used to be
    // built by gluing "Cleric " in front of "Spell save DC", so the capital was
    // an artefact of the concatenation rather than a decision. The whole label
    // is one message now and reads as English.
    expect(cards()).toContain('Cleric spell save DC13')
    expect(cards()).toContain('Wizard spell save DC12')
  })

  it('leads the speed with walking and keeps the rest beside it', () => {
    render({
      ...BASE,
      base: {
        ...BASE.base,
        speeds: [
          { kind: 'flying', distance: 50 },
          { kind: 'walking', distance: 30 },
        ],
      },
    })

    // Walking leads however the projection ordered them: a character with a
    // fly speed still walks, and walking is the number asked for.
    // "Flying", not the wire's "flying": a speed kind is a closed enum the Go
    // model owns, so the client names it out of the catalogue rather than
    // printing the slug. See labels.ts.
    expect(cards()).toContain('Speed30 ft.Flying 50 ft.')
  })

  it('lets the sense name its own card', () => {
    render({ ...BASE, base: { ...BASE.base, senses: [{ kind: 'darkvision', distance: 60 }] } })

    // "Vision 60 ft." would say less than this, and the label is the half
    // with room for the word.
    expect(cards()).toContain('Darkvision60 ft.')
  })

  it('says vision is normal when there is no special sense', () => {
    render(BASE)

    expect(cards()).toContain('VisionNormal')
  })

  it('draws the Hit Dice, which used to sit beside the backpack', () => {
    render({ ...BASE, resources: { hitDice: [{ dice: '3d8', max: 3 }] } })

    expect(cards()).toContain('Hit Dice3d8')
  })

  it('draws temporary hit points as a zero rather than an absence', () => {
    // Nought temporary hit points is a fact about the character -- the shield
    // is down -- where a dash would read as "not tracked".
    render(BASE)
    expect(cards()).toContain('Temp HP0')

    render({ ...BASE, base: { ...BASE.base, hitPoints: { current: 9, max: 12, temporary: 5 } } })
    expect(cards()).toContain('Temp HP5')
  })

  it('starts the second row with the spellcasting numbers', () => {
    render({
      ...BASE,
      status: {
        ...BASE.status,
        spellcasting: [{ class: 'wizard', ability: 'int', saveDC: 12, attackBonus: 4 }],
      },
    })

    // Six to a line, and the row breaks are meaning rather than wrapping: the
    // body's state, then what it can do at range.
    expect(labels()).toEqual([
      'Hit points',
      'Temp HP',
      'Hit Dice',
      'Armor class',
      'Initiative',
      'Proficiency',
      'Spell attack bonus',
      'Spell save DC',
      'Spellcasting ability',
      'Passive Perception',
      'Speed',
      'Vision',
    ])
  })

  it('says so rather than inventing a number it was not sent', () => {
    render(BASE)

    // A speed of "0 ft." would be a claim; a dash is the absence of one.
    expect(cards()).toContain('Speed--')
    expect(cards()).toContain('Hit Dice--')
  })
})
