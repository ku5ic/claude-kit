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
    expect(kit?.text).toContain('comes from a tool result in this session')
  })

  test('switched off, the prompt is the engine sections alone', { options: { precedence: false } }, async ($, on) => {
    engineSections(on)
    const { sections } = await $.prompt.compose(RENDER)
    expect(sections.map(s => s.id)).toEqual(['intro', 'tone'])
  })

  test('every prompt carries the reminder, beside its own context', async ($, on) => {
    engineTakesSubmission(on)
    const first = await $.prompt.submit({ text: 'hello', wait: false, origin: { kind: 'composer' }, context: ['other'] })
    const second = await $.prompt.submit({ text: 'again', wait: false, origin: { kind: 'composer' } })
    expect(first.context?.[0]).toBe('other')
    expect(first.context?.[1]).toMatch(/^kit: .*outrank/)
    expect(first.context?.[1]).toContain('comes from a tool result in this session')
    expect(second.context).toEqual([first.context?.[1]])
  })

  test('switched off, a prompt carries no reminder', { options: { precedence: false } }, async ($, on) => {
    engineTakesSubmission(on)
    const submitted = await $.prompt.submit({ text: 'hello', wait: false, origin: { kind: 'composer' } })
    expect(submitted.context).toBe(undefined)
  })
})

// Stands in for the engine beneath the plugin, which takes the submission as the plugin passed it on.
function engineTakesSubmission(on: On) {
  on('prompt.submit', ($, e) => e)
}
