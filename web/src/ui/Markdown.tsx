import { Box } from '@mantine/core'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

/**
 * Catalogue prose uses Markdown. Raw HTML is never executed.
 *
 * `inline` draws paragraphs as spans, which is what lets a caller clamp the
 * prose to a number of lines: a clamp does not reach into a nested block.
 */
export function Markdown({ children, size = 'sm', inline = false }: { children: string; size?: 'xs' | 'sm'; inline?: boolean }) {
  return (
    <Box component={inline ? 'span' : 'div'} className="markdown-body" fz={size}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml {...(inline ? { components: { p: 'span' } } : {})}>{children}</ReactMarkdown>
    </Box>
  )
}
