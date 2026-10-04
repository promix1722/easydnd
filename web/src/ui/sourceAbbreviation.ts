export function sourceAbbreviation(id: string): string {
  const code = id.split(':').at(-1) ?? id
  return code === 'phb' ? 'PH' : code === 'srd-5.1' ? 'SRD 5.1' : code.toUpperCase()
}
