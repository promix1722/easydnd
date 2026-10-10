import { useState } from 'react'

import type { Change } from '@/lib/api'
import type { CustomOption } from '@/lib/api/characters'
import { type RulesLock } from '@/lib/api/packs'

/** What the player has entered on a build screen and not yet saved. */
export function useBuildDrafts() {
  const [selectedRules, setSelectedRules] = useState<RulesLock | undefined>(undefined)
  const [packError, setPackError] = useState('')
  const [packDirty, setPackDirty] = useState(false)
  const [nameDraft, setNameDraft] = useState('')
  const [imageDraft, setImageDraft] = useState<string | undefined>(undefined)
  const [draftRules, setDraftRules] = useState<Change[] | null>(null)
  // The custom entry being written, opened from a picker's last option or
  // from the entry's own block.
  const [customDraft, setCustomDraft] = useState<CustomOption | null>(null)
  const [nameError, setNameError] = useState<string | undefined>(undefined)
  return {
    selectedRules, setSelectedRules,
    packError, setPackError,
    packDirty, setPackDirty,
    nameDraft, setNameDraft,
    imageDraft, setImageDraft,
    draftRules, setDraftRules,
    customDraft, setCustomDraft,
    nameError, setNameError,
  }
}

export type BuildDrafts = ReturnType<typeof useBuildDrafts>
