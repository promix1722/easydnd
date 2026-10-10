import { screen } from '@testing-library/react'
import { renderAt } from '@/test/render'
import { sourceOptions } from '@/lib/api/catalog'
import { SourceTags } from './SourceTags'

it('shows D&D books without offering SRD as another selected source', () => {
  const provenance = {
    packId: 'dnd-2014', packTitle: 'D&D 2014', version: '2014.2.2', digest: 'test',
    sources: [
      { id: 'dnd-2014:phb', name: 'Player’s Handbook' },
      { id: 'dnd-2014:srd-5.1', name: 'SRD 5.1' },
    ],
  }
  renderAt('desktop', <SourceTags provenance={provenance} />)
  expect(screen.getByText('D&D 2014 v2014.2.2')).toBeInTheDocument()
  expect(screen.getByText('Player’s Handbook')).toBeInTheDocument()
  expect(screen.queryByText('SRD 5.1')).not.toBeInTheDocument()
  const options = sourceOptions([{ slug: 'dnd-2014/barbarian', name: 'Barbarian', provenance }])
  expect(options.packs.map((pack) => pack.id)).toEqual(['dnd-2014'])
  expect(options.sources.map((source) => source.id)).toEqual(['dnd-2014:phb'])
  expect(provenance.sources).toHaveLength(2)
})

it('keeps SRD source tags when the entry belongs to the SRD pack', () => {
  renderAt('desktop', <SourceTags provenance={{
    packId: 'srd-2014', packTitle: 'SRD pack', version: '1.1.0',
    sources: [{ id: 'srd-2014:srd-5.1', name: 'SRD 5.1' }],
  }} />)
  expect(screen.getByText('SRD 5.1')).toBeInTheDocument()
})
