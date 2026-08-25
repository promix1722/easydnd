import { useEffect } from 'react'
import { useNavigate } from 'react-router'

import { createStubCharacter } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import { Button } from '@/ui'

/**
 * Makes the reference character in one press: a finished level-3 half-elf
 * rogue, so that working on the sheet, the log page or the character list does not
 * begin with a walk through five tabs.
 *
 * **A development build's button**, and it lives in its own file so that a
 * production bundle can genuinely drop it. Its one call site is behind
 * `import.meta.env.DEV`, which Vite replaces with a literal, so the branch
 * folds away and this module -- with `createStubCharacter` and the hook it
 * needs -- goes unreferenced and is eliminated. Inlined into the character list it
 * could not be: a hook cannot sit inside a branch, so the call would have to be
 * unconditional and the code would ship to everybody merely unreachable.
 *
 * It goes to the **sheet**, which is the one place it differs from Import. An
 * import answers no prompts, so there is always something left to decide; a
 * stub is finished, so the sheet is the thing worth looking at.
 */
export function StubButton({
  folder,
  onFailed,
}: {
  /** The folder the character list is filtered to, if any. */
  folder?: string | undefined
  /**
   * Reported upward, because the character list owns where errors are drawn. Must
   * be stable across renders; the character list passes a setState.
   */
  onFailed: (message: string | null) => void
}) {
  const navigate = useNavigate()
  const stub = useAction(createStubCharacter)

  // Reported from an effect rather than from the click handler, because the
  // handler closes over the render's `error` and would report the previous
  // value -- null on the first failure, which is the one worth seeing.
  useEffect(() => {
    onFailed(stub.error)
  }, [stub.error, onFailed])

  async function onClick() {
    const created = await stub.run(folder)
    if (created !== null) await navigate(`/characters/${created.id}`)
  }

  return (
    <Button variant="default" loading={stub.pending} onClick={() => void onClick()}>
      Stub
    </Button>
  )
}
