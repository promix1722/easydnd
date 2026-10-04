import { screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import { renderAt } from '@/test/render'
import { Markdown } from './Markdown'
import { joinProse } from './prose'

it('renders emphasis, lists, and tables from spell prose', () => {
  const { container } = renderAt('mobile', <Markdown>{'***Создание воды.*** Вы создаёте воду.\n\n- Первый эффект\n- Второй эффект\n\n| Уровень | Урон |\n| --- | --- |\n| 1 | 1d6 |'}</Markdown>)
  expect(screen.getByText('Создание воды.').closest('strong')).not.toBeNull()
  expect(container.querySelector('em')).toHaveTextContent('Создание воды.')
  expect(screen.getAllByRole('listitem')).toHaveLength(2)
  expect(screen.getByRole('table')).toHaveTextContent('1d6')
  expect(container.textContent).not.toContain('***')
})

it('does not execute embedded HTML or unsafe links', () => {
  const { container } = renderAt('desktop', <Markdown>{'<script>alert(1)</script>\n\n[unsafe](javascript:alert%281%29)\n\n**Safe text**'}</Markdown>)
  expect(container.querySelector('script')).toBeNull()
  expect(container.querySelector('a')).not.toHaveAttribute('href', 'javascript:alert%281%29')
  expect(screen.getByText('Safe text').tagName).toBe('STRONG')
})

it('keeps the rows of a table and the items of a list together when joining prose blocks', () => {
  const { container } = renderAt('mobile', <Markdown>{joinProse(['Intro.', '##### Sizes', '| Size | HP |', '|---|---|', '| Tiny | 20 |', 'Between.', '- one', '- two', 'After.'])}</Markdown>)
  expect(screen.getByRole('table')).toHaveTextContent('Tiny')
  expect(container.querySelectorAll('ul')).toHaveLength(1)
  expect(screen.getAllByRole('listitem')).toHaveLength(2)
  expect(container.querySelectorAll('p')).toHaveLength(3)
  expect(container.textContent).not.toContain('|')
})
