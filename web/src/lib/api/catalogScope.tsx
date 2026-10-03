import { DEFAULT_BUILD_POLICY } from './packPolicy'
import { createContext, useContext } from 'react'
/** A request scope, never a global mutable selection. */
export const CatalogScope = createContext('')
export const useCatalogScope = () => useContext(CatalogScope)

export const RulesEdition = createContext('2014')
export const useRulesEdition = () => useContext(RulesEdition)

export const CharacterPolicy = createContext(DEFAULT_BUILD_POLICY)
export const useCharacterPolicy = () => useContext(CharacterPolicy)
