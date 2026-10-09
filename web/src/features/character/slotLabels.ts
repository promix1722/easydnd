import { ELSEWHERE } from '@/domain'
import type { Slot } from '@/domain'
import { useT } from '@/lib/i18n'

/**
 * A card on the paperdoll. One per slot, except the ring slot, which holds
 * two and is drawn as two cards; and `elsewhere`, drawn only when something
 * equipped has no slot to be shown in.
 */
export type Card = Exclude<Slot, 'ring'> | 'ring:0' | 'ring:1' | typeof ELSEWHERE

/** The player's word for each card. Spelled out key by key so the message check can see them. */
export function useSlotLabels(): Record<Card, string> {
  const t = useT()
  return {
    head: t('equipment.slot.head'), neck: t('equipment.slot.neck'), back: t('equipment.slot.back'), body: t('equipment.slot.body'),
    arms: t('equipment.slot.arms'), waist: t('equipment.slot.waist'), feet: t('equipment.slot.feet'),
    'main-hand': t('equipment.slot.main-hand'), 'off-hand': t('equipment.slot.off-hand'),
    'ring:0': t('equipment.slot.ring1'), 'ring:1': t('equipment.slot.ring2'),
    custom: t('equipment.slot.custom'), elsewhere: t('equipment.slot.elsewhere'),
  }
}
