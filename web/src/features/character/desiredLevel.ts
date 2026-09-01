import type { Change } from '@/lib/api'

/**
 * The change that declares a desired level: one path, one number.
 *
 * Shared by the build screen's form and the sheet's Level up button, so the
 * two ways of saying "take me to level N" cannot drift into writing two
 * different entries.
 */
export function desiredLevelChange(level: number): Change {
  return { path: 'identity.desiredLevel', op: 'set', value: { kind: 'int', int: level } }
}
