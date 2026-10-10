import { useT, type MessageKey } from '@/lib/i18n'
import { AVATAR_CLASSES, AVATAR_RANDOM, Avatar, Card, Page, SimpleGrid, Stack, Text, Title } from '@/ui'

const LABELS: Record<typeof AVATAR_CLASSES[number], MessageKey> = {
  barbarian: 'avatar.class.barbarian',
  bard: 'avatar.class.bard',
  cleric: 'avatar.class.cleric',
  druid: 'avatar.class.druid',
  fighter: 'avatar.class.fighter',
  monk: 'avatar.class.monk',
  paladin: 'avatar.class.paladin',
  ranger: 'avatar.class.ranger',
  rogue: 'avatar.class.rogue',
  sorcerer: 'avatar.class.sorcerer',
  warlock: 'avatar.class.warlock',
  wizard: 'avatar.class.wizard',
}

const RANDOM_LABELS: Record<typeof AVATAR_RANDOM[number], MessageKey> = {
  wolf: 'avatar.random.wolf',
  fox: 'avatar.random.fox',
  owl: 'avatar.random.owl',
  raven: 'avatar.random.raven',
  bear: 'avatar.random.bear',
  lion: 'avatar.random.lion',
  dragon: 'avatar.random.dragon',
  stag: 'avatar.random.stag',
  cat: 'avatar.random.cat',
  serpent: 'avatar.random.serpent',
  turtle: 'avatar.random.turtle',
  bat: 'avatar.random.bat',
  phoenix: 'avatar.random.phoenix',
  griffin: 'avatar.random.griffin',
  kraken: 'avatar.random.kraken',
  skull: 'avatar.random.skull',
  helmet: 'avatar.random.helmet',
  crown: 'avatar.random.crown',
  mask: 'avatar.random.mask',
  crystal: 'avatar.random.crystal',
  key: 'avatar.random.key',
  chalice: 'avatar.random.chalice',
  compass: 'avatar.random.compass',
  moon: 'avatar.random.moon',
}

export function AvatarGalleryScreen() {
  const t = useT()
  return (
    <Page trail={[{ label: t('avatar.gallery') }]} subtitle={t('avatar.galleryHint')}>
      <Stack gap="lg">
        <Title order={2}>{t('avatar.classSet')}</Title>
        <SimpleGrid cols={{ base: 2, sm: 3, lg: 4 }} spacing="md">
          {AVATAR_CLASSES.map((name) => (
            <Card key={name} component="figure" withBorder padding="md" m={0}>
              <Stack align="center" gap="sm">
                <Avatar fallback={`/avatars/${name}.webp`} size={128} />
                <Text component="figcaption" ta="center">{t(LABELS[name])}</Text>
              </Stack>
            </Card>
          ))}
        </SimpleGrid>
        <Title order={2}>{t('avatar.randomSet')}</Title>
        <SimpleGrid cols={{ base: 2, sm: 3, lg: 4 }} spacing="md">
          {AVATAR_RANDOM.map((name) => (
            <Card key={name} component="figure" withBorder padding="md" m={0}>
              <Stack align="center" gap="sm">
                <Avatar fallback={`/avatars/random/${name}.webp`} size={128} />
                <Text component="figcaption" ta="center">{t(RANDOM_LABELS[name])}</Text>
              </Stack>
            </Card>
          ))}
        </SimpleGrid>
      </Stack>
    </Page>
  )
}
