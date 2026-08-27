import { Alert, Badge, Button, Code, Columns, Group, Loader, Stack, Text } from '@/ui'
import { WEB_VERSION } from '@/lib/buildinfo'

import { useApiStatus } from './useApiStatus'

import { useT } from '@/lib/i18n'

/**
 * Shows both halves of a release side by side.
 *
 * The two versions must agree: the frontend and the Go binary ship in the same
 * release directory and are swapped together, so a mismatch here means nginx
 * is serving a stale bundle -- the exact failure the pipeline's version.json
 * check exists to catch.
 */
export function StatusPanel() {
  const t = useT()
  const { data, error, loading, reload } = useApiStatus()

  const mismatch = data !== null && WEB_VERSION !== 'dev' && data.version !== WEB_VERSION

  return (
    <Stack gap="lg">
      <Columns
        cols={2}
        sections={[
          {
            key: 'web',
            title: t('status.frontend'),
            content: (
              <Stack gap="xs">
                <Text size="sm" c="dimmed">
                  {t('status.bundleVersion')}
                </Text>
                <Code>{WEB_VERSION}</Code>
              </Stack>
            ),
          },
          {
            key: 'api',
            title: t('status.api'),
            content: loading ? (
              <Group gap="xs">
                <Loader size="sm" />
                <Text size="sm">{t('status.contacting')}</Text>
              </Group>
            ) : error !== null ? (
              <Alert color="red" title={t('status.unreachable')}>
                {error}
              </Alert>
            ) : (
              <Stack gap="xs">
                <Group gap="xs">
                  <Text size="sm" c="dimmed">
                    {t('status.health')}
                  </Text>
                  <Badge color={data?.health === 'ok' ? 'green' : 'yellow'}>{data?.health}</Badge>
                </Group>
                <Text size="sm" c="dimmed">
                  {t('status.binaryVersion')}
                </Text>
                <Code>{data?.version}</Code>
              </Stack>
            ),
          },
        ]}
      />

      {mismatch && (
        <Alert color="orange" title={t('status.mismatch')}>
          {t('status.mismatchDetail')} <Code>index.html</Code>.
        </Alert>
      )}

      <Group>
        <Button onClick={reload} loading={loading} variant="light">
          {t('status.recheck')}
        </Button>
      </Group>
    </Stack>
  )
}
