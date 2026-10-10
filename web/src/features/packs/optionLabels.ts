import type { Translate, MessageKey } from '@/lib/i18n'
const options: Record<string, MessageKey> = {
  constant: 'packs.option.constant',
  read: 'packs.option.read',
  add: 'packs.option.add',
  subtract: 'packs.option.subtract',
  multiply: 'packs.option.multiply',
  min: 'packs.option.min',
  max: 'packs.option.max',
  'floor-div': 'packs.option.floor-div',
  'ceil-div': 'packs.option.ceil-div',
  eq: 'packs.option.eq',
  gte: 'packs.option.gte',
  lte: 'packs.option.lte',
  and: 'packs.option.and',
  or: 'packs.option.or',
  not: 'packs.option.not',
  grant: 'packs.option.grant',
  set: 'packs.option.set',
  floor: 'packs.option.floor',
  ceil: 'packs.option.ceil',
  sum: 'packs.option.sum',
  all: 'packs.option.all',
  amount: 'packs.option.amount',
  budget: 'packs.option.budget',
  known: 'packs.option.known',
  prepared: 'packs.option.prepared',
  spellbook: 'packs.option.spellbook',
}
export function optionLabel(t: Translate, value: string): string {
  return options[value] ? t(options[value]) : value
}
