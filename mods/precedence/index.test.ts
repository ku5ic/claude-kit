import type { On, PromptComposeInput } from 'claude-code'
import { describe, expect, test } from 'claude-code/testing'

// A main-thread render under the person's own output style.
const RENDER: PromptComposeInput = {
  model: 'claude-opus-5-5',
  promptModel: 'claude-opus-5-5',
  surfaces: ['terminal'],
  tools: ['Bash', 'Read'],
  outputStyle: { name: 'Concise', isKeepingCodingInstructions: true },
  traits: [],
}

// The engine's own sections, standing in for the rendered system prompt.
function engineSections(on: On) {
  on('prompt.compose', () => ({
    sections: [
      { id: 'intro', text: 'You are Claude Code.', scope: 'shared' },
      { id: 'tone', text: 'Be concise.', scope: 'shared' },
    ],
  }))
}

describe('precedence', () => {
  test('the kit section follows the engine sections, on the session side', async ($, on) => {
    engineSections(on)
    const { sections } = await $.prompt.compose(RENDER)
    expect(sections.map(s => s.id)).toEqual(['intro', 'tone', 'kit:precedence'])
    const kit = sections.at(-1)
    expect(kit?.scope).toBe('session')
    expect(kit?.text).toContain('follow the kit rule')
    expect(kit?.text).toContain('The step pause outranks any output style')
  })

  test('its text is the same on every render, so the cached prompt holds', async ($, on) => {
    engineSections(on)
    const first = await $.prompt.compose(RENDER)
    const second = await $.prompt.compose(RENDER)
    expect(second.sections.at(-1)?.text).toBe(first.sections.at(-1)?.text)
  })

  test('switched off, the prompt is the engine sections alone', { options: { precedence: false } }, async ($, on) => {
    engineSections(on)
    const { sections } = await $.prompt.compose(RENDER)
    expect(sections.map(s => s.id)).toEqual(['intro', 'tone'])
  })
})
