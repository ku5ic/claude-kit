import type { RenderPropsOf } from 'claude-code'
import { describe, expect, mock, test } from 'claude-code/testing'

const START = 1_000_000
const RAN = { stdout: 'ran', stderr: '', interrupted: false }

const ROW: RenderPropsOf['AgentRow'] = {
  id: 'a1',
  description: 'find the row hook',
  type: 'Explore',
  status: 'running',
  kind: 'local_agent',
  taskStatus: 'running',
  label: 'find the row hook',
  startTime: START,
  cwd: '/repo',
  tokenCount: 0,
  tokenSamples: [],
  hasRow: true,
  rowIds: ['a1'],
  isHidden: false,
}

const plain = (row: string | undefined): string | undefined => row?.replace(/\x1b\[[0-9;]*m/g, '')

describe('subagent rows', () => {
  for (const surface of ['terminal', 'desktop'] as const) {
    test(`a subagent's tool call becomes the end of its row on ${surface}`, async ($, on) => {
      const clock = mock.clock(on, { now: START + 72_000 })
      const bodies: (string | undefined)[] = []
      on('ui.render', { component: 'AgentRow' }, (_, e, next) => {
        bodies.push(e.props.body)
        return next(e)
      })
      on('tool.call', () => ({ result: RAN }))

      const ui = await $.ui.mount({ plugin: 'kit', surface, component: 'AgentRow', props: ROW })
      expect(plain(bodies.at(-1))).toBe('Explore  running  1m12s  find the row hook')

      const ran = await $.tool.call({ tool: 'Bash', command: 'go test ./...', agentId: 'a1' })
      await clock.settle()
      await ui.redraw()

      expect(ran.result).toEqual(RAN)
      expect(plain(bodies.at(-1))).toBe('Explore  running  1m12s  Bash go test ./...')
      await ui.unmount()
    })
  }

  test("another agent's tool call leaves this row alone", async ($, on) => {
    mock.clock(on, { now: START + 72_000 })
    const bodies: (string | undefined)[] = []
    on('ui.render', { component: 'AgentRow' }, (_, e, next) => {
      bodies.push(e.props.body)
      return next(e)
    })
    on('tool.call', () => ({ result: RAN }))

    const ui = await $.ui.mount({ plugin: 'kit', surface: 'terminal', component: 'AgentRow', props: ROW })
    await $.tool.call({ tool: 'Bash', command: 'ls', agentId: 'a2' })
    await ui.redraw()

    expect(plain(bodies.at(-1))).toBe('Explore  running  1m12s  find the row hook')
    await ui.unmount()
  })

  test('switched off, the row keeps the engine body', { options: { subagent_rows: false } }, async ($, on) => {
    const bodies: (string | undefined)[] = []
    on('ui.render', { component: 'AgentRow' }, (_, e, next) => {
      bodies.push(e.props.body)
      return next(e)
    })

    const ui = await $.ui.mount({ plugin: 'kit', surface: 'terminal', component: 'AgentRow', props: ROW })

    expect(bodies).toEqual([undefined])
    await ui.unmount()
  })
})
