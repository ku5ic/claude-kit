import type { ApiMessage, On, PromptComposeInput, TurnCompleteInput } from 'claude-code'
import { describe, expect, test, type Engine } from 'claude-code/testing'

const ANSWERED: TurnCompleteInput = { reason: 'answer', answer: 'Done.', durationMs: 1, isAborted: false, turnId: 't1' }
const HEAD: ApiMessage = { role: 'user', content: [{ type: 'text', text: 'hello' }] }

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

describe('when the section never reached the model', () => {
  test('compose ran, so the next prompt carries only the reminder', async ($, on) => {
    engineSections(on)
    engineTakesSubmission(on)
    engineEndsTurns(on)
    conversation(on, [HEAD])
    await $.prompt.compose(RENDER)
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('an answered turn with no compose puts the section on the next prompt, once', async ($, on) => {
    engineTakesSubmission(on)
    engineEndsTurns(on)
    conversation(on, [HEAD])
    expect(await contextOf($)).toEqual([REMINDER])
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    expect(await contextOf($)).toEqual([REMINDER])
    expect(await contextOf($)).toEqual([REMINDER])
  })

  for (const reason of ['aborted', 'error'] as const) {
    test(`a turn that ends ${reason} leaves the next prompt alone`, async ($, on) => {
      engineTakesSubmission(on)
      engineEndsTurns(on)
      conversation(on, [HEAD])
      await $.turn.complete({ ...ANSWERED, reason, isAborted: reason === 'aborted' })
      expect(await contextOf($)).toEqual([REMINDER])
    })
  }

  test("a subagent's answered turn leaves the next prompt alone", async ($, on) => {
    engineTakesSubmission(on)
    engineEndsTurns(on)
    conversation(on, [HEAD])
    await $.turn.complete({ ...ANSWERED, agentId: 'a1' })
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a slash command leaves the section for the next prompt', async ($, on) => {
    engineTakesSubmission(on)
    engineEndsTurns(on)
    conversation(on, [HEAD])
    await $.turn.complete(ANSWERED)
    expect(await contextOf($, { command: { name: 'kit-stats', args: '' } })).toEqual([REMINDER])
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
  })

  test('a dropped prompt leaves the section for the next one', async ($, on) => {
    let isDropping = true
    on('prompt.submit', (_, e) => (isDropping ? { drop: 'no' } : e))
    engineEndsTurns(on)
    conversation(on, [HEAD])
    await $.turn.complete(ANSWERED)
    await contextOf($)
    isDropping = false
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
  })

  test('a compaction brings the section back, with more messages than before it', async ($, on) => {
    engineTakesSubmission(on)
    engineEndsTurns(on)
    const messages = conversation(on, [summary('first')])
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    messages.splice(0, 1, summary('second'), HEAD, HEAD)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a window that slid at the 4096 cap is no compaction', async ($, on) => {
    engineTakesSubmission(on)
    engineEndsTurns(on)
    const messages = conversation(on, [HEAD])
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    messages.splice(0, 1, ...Array.from({ length: 4096 }, () => summary('slid')))
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('an empty conversation waits for one with a first message', async ($, on) => {
    engineTakesSubmission(on)
    engineEndsTurns(on)
    const messages = conversation(on, [])
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([REMINDER])
    messages.push(HEAD)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
  })

  test('a failed read still sends the reminder', async ($, on) => {
    engineTakesSubmission(on)
    engineEndsTurns(on)
    on('session.messages', () => {
      throw new Error('no messages')
    })
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a failed state write still leaves the section in the system prompt', async ($, on) => {
    engineSections(on)
    failStateWrites(on)
    const { sections } = await $.prompt.compose(RENDER)
    expect(sections.map(s => s.id)).toEqual(['intro', 'tone', 'kit:precedence'])
  })

  test('a failed state write after delivery submits the prompt once, with the section', async ($, on) => {
    let submissions = 0
    on('prompt.submit', (_, e) => {
      submissions++
      return e
    })
    engineEndsTurns(on)
    conversation(on, [HEAD])
    const writes = failStateWrites(on, { isFailing: false })
    await $.turn.complete(ANSWERED)
    writes.isFailing = true
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    expect(submissions).toBe(1)
  })

  test('switched off, an answered turn adds nothing', { options: { precedence: false } }, async ($, on) => {
    engineTakesSubmission(on)
    engineEndsTurns(on)
    conversation(on, [HEAD])
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toBe(undefined)
  })
})

// Stands in for the engine beneath the plugin, which takes the submission as the plugin passed it on.
function engineTakesSubmission(on: On) {
  on('prompt.submit', ($, e) => e)
}

function engineEndsTurns(on: On) {
  on('turn.complete', (_, e) => ({ text: e.answer }))
}

function failStateWrites(on: On, writes = { isFailing: true }) {
  on('state.set', (_, e, next) => (writes.isFailing ? { deny: 'no state' } : next(e)))
  return writes
}

// The conversation as the engine would send it; a test changes the array it returns.
function conversation(on: On, messages: ApiMessage[]): ApiMessage[] {
  on('session.messages', () => ({ value: [...messages] }))
  return messages
}

async function contextOf($: Engine, extra: Partial<Parameters<Engine['prompt']['submit']>[0]> = {}) {
  const submitted = await $.prompt.submit({ text: 'go on', wait: false, origin: { kind: 'composer' }, ...extra })
  return submitted.context
}

// The two texts by what opens them, since the mod keeps both to itself.
const SECTION = expect.stringMatching(/^Every fact you show the user/)
const REMINDER = expect.stringMatching(/^kit: Every fact/)

// Two compaction summaries open with the same text.
function summary(body: string): ApiMessage {
  const opening = 'This session is being continued from a previous conversation that ran out of context. '.repeat(3)
  return { role: 'user', content: [{ type: 'text', text: opening + body }] }
}
