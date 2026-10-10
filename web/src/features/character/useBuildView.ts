import { getEvents, getPrompts, getSheet } from '@/lib/api'
import { characterPath } from '@/lib/api/characters'
import { useResource } from '@/lib/useResource'
import type { Resource } from '@/lib/useResource'

import { EMPTY_VIEW } from './buildModel'
import type { BuildView } from './buildModel'
import { resolveRefNames } from './refNames'

/**
 * Everything one build screen reads, in one round of requests.
 *
 * Three requests, deliberately: see BuildScreen for the three questions they
 * answer.
 */
export function useBuildView(id: string): Resource<BuildView> {
  // useResource refreshes on translator changes without unmounting the active draft.
  return useResource<BuildView>(`build:${id}`, async (signal) => {
    if (id === '') return EMPTY_VIEW
    const [prompts, log, sheet] = await Promise.all([
      getPrompts(id, signal),
      getEvents(id, signal),
      getSheet(id, signal),
    ])
    return {
      prompts,
      rules: log.rules,
      events: log.events,
      sheet,
      names: await resolveRefNames([
        ...log.events, ...prompts.prompts,
        // An alignment is written as a value at a path, not as a reference,
        // so nothing above asks for its name.
        ...log.events.flatMap((event) => (event.changes ?? [])
          .filter((change) => change.path === 'identity.alignment' && change.value.slug !== undefined)
          .map((change) => ({ ref: `alignment:${change.value.slug}` }))),
        ...(prompts.spellRules ?? []).flatMap((rule) => [{ source: rule.source }, ...(rule.automatic ?? []).map((slug) => ({ ref: `spell:${slug}` })), ...(rule.listClasses ?? []).map((slug) => ({ ref: `class:${slug}` }))]),
        // An equipment card is titled by what it offers, so those items are
        // named before any card is opened.
        ...prompts.prompts.filter((prompt) => prompt.choice.kind === 'equipment')
          .flatMap((prompt) => [
            // The category a card or one of its options draws on, by its
            // catalogue name: "Arcane Foci" in English is not a title in Russian.
            ...(prompt.choice.from.category === undefined ? [] : [{ ref: `equipment-category:${prompt.choice.from.category}` }]),
            ...(prompt.choice.from.options ?? []).flatMap((option) => [option, ...(option.items ?? [])]).flatMap((option) => [
              ...(option.ref === undefined ? [] : [{ ref: option.ref }]),
              ...(option.choice?.from.category === undefined ? [] : [{ ref: `equipment-category:${option.choice.from.category}` }]),
            ]),
          ]),
        ...[...sheet.equipment.equipped, ...sheet.equipment.backpack, ...sheet.equipment.loot]
          .flatMap((stack) => stack.item === undefined ? [] : [{ ref: `item:${stack.item}` }]),
      ], `${characterPath(id)}/catalog`),
    }
  })
}
