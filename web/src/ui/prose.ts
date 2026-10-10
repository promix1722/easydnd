/**
 * Joins a catalogue entry's `desc` or `blocks.*` array into one Markdown
 * document. Each element is one line-level block (docs/packs.md, "Prose
 * formats"): a paragraph, a heading, a single list item or a single table row.
 * Paragraphs want a blank line between them; the rows of one table and the
 * items of one list must sit on consecutive lines, or GFM reads every row as a
 * paragraph of pipes.
 */
export function joinProse(blocks: readonly string[]): string {
  const kind = (block: string) => (block.startsWith('|') ? 'row' : /^[-*] /.test(block) ? 'item' : 'text')
  return blocks.reduce((out, block, i) => {
    if (i === 0) return block
    const same = kind(block) !== 'text' && kind(block) === kind(blocks[i - 1]!)
    return out + (same ? '\n' : '\n\n') + block
  }, '')
}
