import type { Prompt } from '@/lib/api'

/** All server choices are supported; category choices arrive with their legal members. */
export function creationPrompt(prompt: Prompt): Prompt { return prompt }
