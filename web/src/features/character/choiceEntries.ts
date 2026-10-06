import { bySlug, getCollection, getEntries } from '@/lib/api'
import type { Entry, OptionSet, Prompt } from '@/lib/api'
import { collectionOfKind, kindOf, slugOf } from '@/domain'

export async function loadEntries(prompt: Prompt, scope = ''): Promise<Map<string, Entry>> {
  const whole = new Set<string>()
  const wanted = new Map<string, Set<string>>()

  const visitSet = (set: OptionSet) => {
    if (set.category !== undefined) {
      const categories = wanted.get('equipment-categories') ?? new Set<string>()
      categories.add(set.category)
      wanted.set('equipment-categories', categories)
    }
    if (set.kind === 'collection' && set.collection !== undefined) {
      const collection = collectionOfKind(set.collection)
      // Never spells: that collection is not served whole. The server lists
      // a spell choice's options inline, so this is only a guard.
      if (collection !== null && collection !== 'spells') whole.add(collection)
      return
    }
    for (const option of set.options ?? []) {
      if (option.kind === 'ref' && option.ref !== undefined) {
        const collection = collectionOfKind(kindOf(option.ref))
        if (collection === null) continue
        const bucket = wanted.get(collection) ?? new Set<string>()
        bucket.add(slugOf(option.ref))
        wanted.set(collection, bucket)
      }
      if (option.items !== undefined) visitSet({ kind: 'explicit', options: option.items })
      if (option.choice !== undefined) visitSet(option.choice.from)
    }
  }
  visitSet(prompt.choice.from)

  if (wanted.has('spells')) {
    whole.add('magic-schools')
    whole.add('classes')
  }
  const loaded = await Promise.all([
    ...[...whole].map((collection) => getCollection<Entry>(collection, scope)),
    ...[...wanted].map(([collection, slugs]) => getEntries<Entry>(collection, [...slugs], scope)),
  ])
  const entries = bySlug(loaded.flat())
  const contents = loaded.flat().flatMap((entry) => ((entry as import('@/lib/api').Item).gear?.contents ?? []).map((item) => item.item)).filter((slug) => !entries.has(slug))
  if (wanted.has('equipment')) {
    for (const collection of ['damage-types', 'weapon-properties']) {
      for (const entry of await getCollection<Entry>(collection, scope)) entries.set(entry.slug, entry)
    }
  }
  const items = await getEntries<Entry>('equipment', [...new Set(contents)], scope)
  for (const item of items) entries.set(item.slug, item)
  return entries
}
