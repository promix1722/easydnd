import { useEffect, useLayoutEffect, useRef, useState } from 'react'

import type { AgentEvent } from '@/lib/api/agent'

/**
 * Where the page is in the conversation: the chat's frame sized to the
 * window, the page kept at the end while the reader is there, and whether
 * they have scrolled away from it.
 */
export function useChatScroll(reveal: { events: AgentEvent[]; settled: boolean }, status: string | undefined) {
  const frame = useRef<HTMLDivElement>(null)
  const end = useRef<HTMLDivElement>(null)
  const follow = useRef(true)
  // The page follows the conversation for as long as the reader is at the
  // end of it, and stops the moment they scroll up to read something.
  //
  // Only scrolling *up* stops it. Being far from the end does not: a bubble
  // taller than any threshold arrives in one step, and the smooth scroll that
  // follows it reports every position on the way down -- taking those for the
  // reader's would stop following exactly when there was most to follow.
  const [away, setAway] = useState(false)
  useEffect(() => {
    let last = window.scrollY
    const scrolled = () => {
      const page = document.documentElement
      const y = window.scrollY
      if (page.scrollHeight - y - window.innerHeight < 80) follow.current = true
      else if (y < last) follow.current = false
      last = y
      setAway(!follow.current)
    }
    window.addEventListener('scroll', scrolled, { passive: true })
    return () => window.removeEventListener('scroll', scrolled)
  }, [])
  // Text is laid out after it is rendered -- Markdown, fonts, a wrapping
  // line -- so the end moves without anything React knows of having changed.
  // Watching the page's height is what keeps it at the end regardless, from
  // the moment it is opened -- the page's and not the chat's, because a notice
  // appearing above the chat moves the end just as far as a message does.
  useEffect(() => {
    const grown = new ResizeObserver(() => {
      if (follow.current) end.current?.scrollIntoView?.({ block: 'end' })
    })
    grown.observe(document.body)
    return () => grown.disconnect()
  }, [])
  // The chat is as tall as the window allows from its first moment, so the
  // message box is where it will be at the end of a long conversation: at the
  // foot of the window, with the page's own margin under it. A chat that has
  // outgrown the window scrolls to the same place.
  //
  // Measured rather than written as a sum, because what stands above the chat
  // is not this screen's to know: the shell's header, the page's heading
  // (gone on a phone), a notice. What is under it is the shell's padding,
  // which on a phone is the safe area's.
  useLayoutEffect(() => {
    const box = frame.current
    if (!box) return
    const fit = () => {
      const under = parseFloat(getComputedStyle(box.closest('main') ?? box).paddingBottom) || 0
      const over = box.getBoundingClientRect().top + window.scrollY
      box.style.minHeight = `calc(100dvh - ${Math.round(over + under)}px)`
    }
    fit()
    const moved = new ResizeObserver(fit)
    moved.observe(document.body)
    window.addEventListener('resize', fit)
    return () => {
      moved.disconnect()
      window.removeEventListener('resize', fit)
    }
  }, [])
  const typedTo = reveal.events.at(-1)?.text?.length
  useEffect(() => {
    if (follow.current) end.current?.scrollIntoView?.({ behavior: 'smooth', block: 'end' })
  }, [reveal.events.length, typedTo, reveal.settled, status])
  /** Back to the end, and following it again. */
  const toEnd = () => {
    follow.current = true
    setAway(false)
    end.current?.scrollIntoView?.({ behavior: 'smooth', block: 'end' })
  }
  return { frame, end, follow, away, toEnd }
}
