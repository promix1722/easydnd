import { useState } from 'react'
import { useNavigate } from 'react-router'

import { createCharacter } from '@/lib/api'
import { useAction } from '@/lib/useAction'
import {
  Alert,
  Button,
  Group,
  NumberInput,
  Select,
  SimpleGrid,
  Stack,
  Text,
  TextInput,
  Title,
} from '@/ui'

/**
 * The six abilities, in the order every character sheet prints them.
 *
 * Hardcoded rather than fetched. They are the one part of the compendium that
 * cannot change: a sixth-and-a-half ability would be a different game, and
 * waiting on a round trip to draw six labelled number inputs would be a worse
 * first screen for no benefit.
 */
const ABILITIES = [
  { key: 'str', label: 'Strength' },
  { key: 'dex', label: 'Dexterity' },
  { key: 'con', label: 'Constitution' },
  { key: 'int', label: 'Intelligence' },
  { key: 'wis', label: 'Wisdom' },
  { key: 'cha', label: 'Charisma' },
] as const

/** The standard array, as the SRD prints it. */
const STANDARD_ARRAY = [15, 14, 13, 12, 10, 8]

const METHODS = [
  { value: 'standard-array', label: 'Standard array' },
  { value: 'point-buy', label: 'Point buy' },
  { value: 'rolled', label: 'Rolled' },
  { value: 'manual', label: 'Manual' },
]

type Scores = Record<string, number>

function standardArrayScores(): Scores {
  const out: Scores = {}
  ABILITIES.forEach((ability, i) => {
    out[ability.key] = STANDARD_ARRAY[i] ?? 10
  })
  return out
}

/**
 * Starts a character.
 *
 * The scores entered here are the *base* array, before any racial bonus. The
 * server applies those on every projection, which is what lets the player
 * change their mind about a race later without re-entering six numbers -- so
 * this screen must not "helpfully" pre-add them.
 */
export function CreateCharacterScreen() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [method, setMethod] = useState('standard-array')
  const [scores, setScores] = useState<Scores>(standardArrayScores)

  const create = useAction(createCharacter)

  const submit = async () => {
    const created = await create.run({ name: name.trim(), method, abilities: scores })
    if (created) await navigate(`/characters/${created.id}/build`)
  }

  const nameError = create.fields.find((f) => f.field === 'name')?.message

  return (
    <Stack gap="lg" maw={640}>
      <div>
        <Title order={2}>New character</Title>
        <Text c="dimmed" size="sm">
          Name and ability scores. Everything else comes next, one question at a time.
        </Text>
      </div>

      {create.error !== null && create.fields.length === 0 && (
        <Alert color="red" title="Could not create the character">
          {create.error}
        </Alert>
      )}

      <TextInput
        label="Name"
        placeholder="Who are they?"
        value={name}
        error={nameError}
        onChange={(event) => setName(event.currentTarget.value)}
      />

      <Select
        label="How were the scores generated?"
        description="Recorded so that coming back to this step reopens the right editor."
        data={METHODS}
        value={method}
        allowDeselect={false}
        onChange={(value) => {
          if (value === null) return
          setMethod(value)
          if (value === 'standard-array') setScores(standardArrayScores())
        }}
      />

      <div>
        <Text size="sm" fw={500}>
          Ability scores
        </Text>
        <Text size="xs" c="dimmed" mb="xs">
          The base array. Racial bonuses are added by the rules, not here.
        </Text>
        <SimpleGrid cols={{ base: 2, sm: 3 }} spacing="sm">
          {ABILITIES.map((ability) => (
            <NumberInput
              key={ability.key}
              label={ability.label}
              min={1}
              max={30}
              value={scores[ability.key] ?? 10}
              error={create.fields.find((f) => f.field === `abilities.${ability.key}`)?.message}
              onChange={(value) =>
                setScores((current) => ({
                  ...current,
                  [ability.key]: typeof value === 'number' ? value : Number(value) || 10,
                }))
              }
            />
          ))}
        </SimpleGrid>
      </div>

      <Group>
        <Button onClick={() => void submit()} loading={create.pending} disabled={name.trim() === ''}>
          Create and continue
        </Button>
        <Button variant="subtle" onClick={() => void navigate('/')}>
          Cancel
        </Button>
      </Group>
    </Stack>
  )
}
