import { Box } from '@mantine/core'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

/** Catalogue prose uses Markdown. Raw HTML is never executed. */
export function Markdown({ children, size = 'sm' }: { children: string; size?: 'xs' | 'sm' }) {
  return (
    <Box className="markdown-body" fz={size}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml>{children}</ReactMarkdown>
    </Box>
  )
}
