interface BookProvenance {
  packId: string
  sources: { id: string; name: string }[]
}

/** SRD membership is an attribution, not an additional pack selected for a character. */
export function bookSources(provenance: BookProvenance) {
  return provenance.sources.filter((source) => provenance.packId === 'srd-2014'
    || source.id.split(':').at(-1) !== 'srd-5.1')
}
