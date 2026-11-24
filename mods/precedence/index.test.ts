import type { ApiMessage, On, PromptComposeInput, RenderSurface, TurnCompleteInput } from 'claude-code'
import { describe, expect, test, type Engine } from 'claude-code/testing'

const ANSWERED: TurnCompleteInput = { reason: 'answer', answer: 'Done.', durationMs: 1, isAborted: false, turnId: 't1' }

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
  engineMachine(on)
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
    expect(kit?.text).toContain('is verified against the code or source itself, so')
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
    expect(first.context?.[1]).toContain('is verified against the code or source itself, so')
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
    engineHolds(on)
    await $.prompt.compose(RENDER)
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('an answered turn with no compose puts the section on the next prompt, once', async ($, on) => {
    engineHolds(on)
    expect(await contextOf($)).toEqual([REMINDER])
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([REMINDER])
    expect(await contextOf($)).toEqual([REMINDER])
  })

  for (const reason of ['aborted', 'error'] as const) {
    test(`a turn that ends ${reason} leaves the next prompt alone`, async ($, on) => {
      engineHolds(on)
      await $.turn.complete({ ...ANSWERED, reason, isAborted: reason === 'aborted' })
      expect(await contextOf($)).toEqual([REMINDER])
    })

    test(`a turn that ends ${reason} after one with no compose keeps the section due`, async ($, on) => {
      engineHolds(on)
      await $.turn.complete(ANSWERED)
      await $.turn.complete({ ...ANSWERED, reason, isAborted: reason === 'aborted' })
      expect(await contextOf($)).toEqual([SECTION, REMINDER])
    })
  }

  test("a subagent's answered turn leaves the next prompt alone", async ($, on) => {
    engineHolds(on)
    await $.turn.complete({ ...ANSWERED, agentId: 'a1' })
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test("a subagent's end keeps the main turn's compose", async ($, on) => {
    engineSections(on)
    engineHolds(on)
    await $.prompt.compose(RENDER)
    await $.turn.complete({ ...ANSWERED, agentId: 'a1' })
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a compose in an earlier turn says nothing about a later one', async ($, on) => {
    engineSections(on)
    engineHolds(on)
    await $.prompt.compose(RENDER)
    await $.turn.complete(ANSWERED)
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
  })

  test('a turn with a compose ends the fallback', async ($, on) => {
    engineSections(on)
    engineHolds(on)
    await $.turn.complete(ANSWERED)
    await $.prompt.compose(RENDER)
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test("a skill's prompt carries the section, so the next one doesn't", async ($, on) => {
    engineHolds(on)
    await $.turn.complete(ANSWERED)
    expect(await contextOf($, { command: { name: 'kit:audit-ci', args: '' } })).toEqual([SECTION, REMINDER])
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a prompt that was queued and cancelled leaves the section for the next one', async ($, on) => {
    const engine = engineHolds(on)
    await $.turn.complete(ANSWERED)
    engine.isKeeping = false
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    engine.isKeeping = true
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a compaction brings the section back, once', async ($, on) => {
    const engine = engineHolds(on)
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    engine.messages.splice(0, engine.messages.length, says('user', 'The conversation so far, in short.'))
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('an assistant that quoted the section has not received it', async ($, on) => {
    const engine = engineHolds(on)
    await $.turn.complete(ANSWERED)
    const [section] = (await contextOf($)) ?? []
    engine.messages.splice(0, engine.messages.length, says('assistant', section ?? ''))
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
    on('state.set', () => ({ deny: 'no state' }))
    const { sections } = await $.prompt.compose(RENDER)
    expect(sections.map(s => s.id)).toEqual(['intro', 'tone', 'kit:precedence'])
  })

  test('switched off, an answered turn adds nothing', { options: { precedence: false } }, async ($, on) => {
    engineHolds(on)
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toBe(undefined)
  })
})

describe('what an earlier session on the machine recorded', () => {
  test('a recorded skip puts the section on the first prompt, once', async ($, on) => {
    engineHolds(on)
    machineRecorded(on, true)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('with no skip recorded, the first prompt carries only the reminder', async ($, on) => {
    engineHolds(on)
    machineRecorded(on, false)
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a first turn that was aborted leaves the record in charge', async ($, on) => {
    const engine = engineHolds(on)
    machineRecorded(on, true)
    engine.isKeeping = false
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
    engine.isKeeping = true
    await $.turn.complete({ ...ANSWERED, reason: 'aborted', isAborted: true })
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
  })

  test('after an answered turn, this session decides', async ($, on) => {
    engineSections(on)
    engineHolds(on)
    const machine = machineRecorded(on, false)
    await $.prompt.compose(RENDER)
    await $.turn.complete(ANSWERED)
    machine.skipped = true
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a turn with a compose clears the record', async ($, on) => {
    engineSections(on)
    engineHolds(on)
    const machine = machineRecorded(on, true)
    await $.prompt.compose(RENDER)
    await $.turn.complete(ANSWERED)
    expect(machine.skipped).toBe(false)
  })

  test('an answered turn with no compose records the skip', async ($, on) => {
    engineHolds(on)
    const machine = machineRecorded(on, undefined)
    await $.turn.complete(ANSWERED)
    expect(machine.skipped).toBe(true)
  })

  test('a record that already says so is not written again', async ($, on) => {
    engineSections(on)
    engineHolds(on)
    const machine = machineRecorded(on, undefined)
    await $.prompt.compose(RENDER)
    await $.turn.complete(ANSWERED)
    expect(machine.writes).toBe(0)
    await $.turn.complete(ANSWERED)
    await $.turn.complete(ANSWERED)
    expect(machine.writes).toBe(1)
  })

  for (const skipped of [true, false]) {
    test(`a subagent's turn leaves a record of ${skipped} alone`, async ($, on) => {
      engineHolds(on)
      const machine = machineRecorded(on, skipped)
      await $.turn.complete({ ...ANSWERED, agentId: 'a1' })
      expect(machine.skipped).toBe(skipped)
      expect(machine.writes).toBe(0)
    })
  }

  test('a headless run with no compose records nothing, and its next prompt carries the section', async ($, on) => {
    engineHolds(on)
    const machine = machineRecorded(on, false)
    machine.surfaces = []
    await $.turn.complete(ANSWERED)
    expect(machine.skipped).toBe(false)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
  })

  test('once a compose ran, a prompt in the first turn ignores the record', async ($, on) => {
    engineSections(on)
    engineHolds(on)
    const machine = machineRecorded(on, false)
    await $.prompt.compose(RENDER)
    machine.skipped = true
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('once a compose ran, a prompt after an aborted first turn ignores the record', async ($, on) => {
    engineSections(on)
    engineHolds(on)
    const machine = machineRecorded(on, false)
    await $.prompt.compose(RENDER)
    await $.turn.complete({ ...ANSWERED, reason: 'aborted', isAborted: true })
    machine.skipped = true
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a failed read of the record still sends the reminder', async ($, on) => {
    engineHolds(on)
    machineRecorded(on, true).failing = 'get'
    expect(await contextOf($)).toEqual([REMINDER])
  })

  test('a failed write of the record still leaves the section in the system prompt', async ($, on) => {
    engineSections(on)
    machineRecorded(on, true).failing = 'set'
    const { sections } = await $.prompt.compose(RENDER)
    expect(sections.map(s => s.id)).toEqual(['intro', 'tone', 'kit:precedence'])
  })

  test('a failed write of the record still puts the section on the next prompt', async ($, on) => {
    engineHolds(on)
    machineRecorded(on, false).failing = 'set'
    await $.turn.complete(ANSWERED)
    expect(await contextOf($)).toEqual([SECTION, REMINDER])
  })

  test('switched off, a recorded skip adds nothing', { options: { precedence: false } }, async ($, on) => {
    engineHolds(on)
    machineRecorded(on, true)
    expect(await contextOf($)).toBe(undefined)
  })
})

type Machine = { skipped?: boolean; writes: number; surfaces: RenderSurface[]; failing?: 'get' | 'set' }

// One machine per test, since more than one helper stands in for it.
const machines = new WeakMap<On, Machine>()

// Stands in for the machine's store, which every session shares, and for where the session draws.
function engineMachine(on: On): Machine {
  const known = machines.get(on)
  if (known) return known
  const machine: Machine = { writes: 0, surfaces: ['terminal'] }
  machines.set(on, machine)
  on('store.get', () => (machine.failing === 'get' ? { deny: 'no store' } : { value: machine.skipped }))
  on('store.set', (_, e) => {
    if (machine.failing === 'set') return { deny: 'no store' }
    machine.skipped = e.value === true
    machine.writes++
    return { value: undefined }
  })
  on('session.surfaces', () => ({ value: machine.surfaces }))
  return machine
}

function machineRecorded(on: On, skipped: boolean | undefined): Machine {
  return Object.assign(engineMachine(on), { skipped })
}

// Stands in for the engine beneath the plugin, which takes the submission as the plugin passed it on.
function engineTakesSubmission(on: On) {
  engineMachine(on)
  on('prompt.submit', ($, e) => e)
}

function engineEndsTurns(on: On) {
  on('turn.complete', (_, e) => ({ text: e.answer }))
}

// Stands in for an engine that ends turns and holds the conversation, each prompt's context in it
// while isKeeping.
function engineHolds(on: On) {
  const engine = { messages: [] as ApiMessage[], isKeeping: true }
  engineMachine(on)
  on('prompt.submit', (_, e) => {
    if (engine.isKeeping) engine.messages.push(says('user', e.text, ...(e.context ?? [])))
    return e
  })
  on('session.messages', () => ({ value: [...engine.messages] }))
  engineEndsTurns(on)
  return engine
}

function says(role: ApiMessage['role'], ...texts: string[]): ApiMessage {
  return { role, content: texts.map(text => ({ type: 'text', text })) }
}

async function contextOf($: Engine, extra: Partial<Parameters<Engine['prompt']['submit']>[0]> = {}) {
  const submitted = await $.prompt.submit({ text: 'go on', wait: false, origin: { kind: 'composer' }, ...extra })
  return submitted.context
}

// The two texts by what opens them, since the mod keeps both to itself.
const SECTION = expect.stringMatching(/^Every fact you show the user/)
const REMINDER = expect.stringMatching(/^kit: Every fact/)
