import { Group, Title } from '@/ui'

/**
 * The mark and the name, top left of every header.
 *
 * One component rather than a `<Title>` repeated in three shells, because the
 * corner a visitor uses to know where they are should not drift between the
 * logged-out header, the desktop navbar shell and the phone tab shell.
 *
 * The icon is the same d20 as `public/favicon.svg` and the installed PWA's
 * icons -- served as a static asset rather than inlined, so the browser reuses
 * the copy it already fetched for the tab.
 */
export function Wordmark({ order = 3 }: { order?: 3 | 4 }) {
  return (
    <Group gap="xs" wrap="nowrap">
      {/* Decorative: the name is right beside it, so announcing the image
          again would only make a screen reader say easydnd twice. */}
      <img src="/favicon.svg" alt="" width={order === 3 ? 28 : 24} height={order === 3 ? 28 : 24} />
      <Title order={order}>easydnd</Title>
    </Group>
  )
}
