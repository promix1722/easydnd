import type { Prompt } from '@/lib/api'

export interface SpellAllowance {
  prompt: Prompt
  eligible: readonly string[]
}

/** Match the combined draft to legal acquisition slots, keeping level rules off the UI. */
export function allocateSpells(allowances: readonly SpellAllowance[], picks: readonly string[]): Map<string, string[]> | null {
  const slots = allowances.flatMap((allowance, index) => Array.from({ length: allowance.prompt.choice.choose }, () => index))
  if (picks.length > slots.length) return null
  const occupants = new Map<number, string>()
  const assign = (pick: string, visited: Set<number>): boolean => {
    for (const [slot, index] of slots.entries()) {
      if (visited.has(slot) || !allowances[index]?.eligible.includes(pick)) continue
      visited.add(slot)
      const previous = occupants.get(slot)
      if (previous === undefined || assign(previous, visited)) {
        occupants.set(slot, pick)
        return true
      }
    }
    return false
  }
  if (new Set(picks).size !== picks.length) return null
  for (const pick of picks) if (!assign(pick, new Set())) return null
  return new Map(allowances.map((allowance, index) => [allowance.prompt.choice.prompt,
    picks.filter((pick) => [...occupants].some(([slot, value]) => slots[slot] === index && value === pick)),
  ]))
}

export interface SpellLevelLimit {
  source: string
  purpose: string
  level: number
  remaining: number
}

/** A flow gate shares the highest-level quota across a class's acquisition prompts. */
export function allocateLimitedSpells(allowances: readonly SpellAllowance[], picks: readonly string[], limits: readonly SpellLevelLimit[], levels: ReadonlyMap<string, number>): Map<string, string[]> | null {
  if (limits.length === 0) return allocateSpells(allowances, picks)
  if (new Set(picks).size !== picks.length) return null
  type Edge = { to: number; capacity: number; reverse: number }
  const graph: Edge[][] = []
  const node = () => { graph.push([]); return graph.length - 1 }
  const edge = (from: number, to: number, capacity: number) => {
    const forward = { to, capacity, reverse: graph[to]!.length }
    graph[from]!.push(forward)
    graph[to]!.push({ to: from, capacity: 0, reverse: graph[from]!.length - 1 })
    return forward
  }
  const start = node(), end = node()
  const targets = allowances.map(({ prompt }) => {
    const target = node()
    edge(target, end, prompt.choice.choose)
    return target
  })
  const gates = limits.map((limit) => {
    const input = node(), output = node()
    edge(input, output, Math.max(0, limit.remaining))
    const destinations = allowances.flatMap(({ prompt }, i) => prompt.source === limit.source && prompt.purpose === limit.purpose
      ? [{ index: i, edge: edge(output, targets[i]!, prompt.choice.choose) }] : [])
    return { input, destinations }
  })
  const routes = picks.map((pick) => {
    const source = node()
    edge(start, source, 1)
    const seen = new Set<number>()
    return allowances.flatMap(({ prompt, eligible }, i) => {
      if (!eligible.includes(pick)) return []
      const gate = limits.findIndex((limit) => limit.source === prompt.source && limit.purpose === prompt.purpose && levels.get(pick) === limit.level)
      if (gate < 0) return [{ edge: edge(source, targets[i]!, 1), index: i, gate: -1 }]
      if (seen.has(gate)) return []
      seen.add(gate)
      return [{ edge: edge(source, gates[gate]!.input, 1), index: -1, gate }]
    })
  })
  for (let flow = 0; flow < picks.length; flow++) {
    const visited = new Set<number>()
    const augment = (from: number): boolean => {
      if (from === end) return true
      visited.add(from)
      for (const path of graph[from]!) {
        if (path.capacity <= 0 || visited.has(path.to) || !augment(path.to)) continue
        path.capacity--
        graph[path.to]![path.reverse]!.capacity++
        return true
      }
      return false
    }
    if (!augment(start)) return null
  }
  const result = new Map(allowances.map(({ prompt }) => [prompt.choice.prompt, [] as string[]]))
  routes.forEach((paths, i) => {
    const used = paths.find((path) => path.edge.capacity === 0)!
    let index = used.index
    if (used.gate >= 0) {
      const destination = gates[used.gate]!.destinations.find((item) => graph[item.edge.to]![item.edge.reverse]!.capacity > 0)!
      graph[destination.edge.to]![destination.edge.reverse]!.capacity--
      index = destination.index
    }
    result.get(allowances[index]!.prompt.choice.prompt)!.push(picks[i]!)
  })
  return result
}
