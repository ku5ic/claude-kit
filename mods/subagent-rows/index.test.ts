import type { On, RenderPropsOf } from 'claude-code'
import { describe, expect, mock, test, type Engine } from 'claude-code/testing'

const START = 1_000_000
const RAN = { stdout: 'ran', stderr: '', interrupted: false }
const IDLE = 'Explore  running  1m12s  find the row hook'

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

// Records every body drawn for an AgentRow, with the clock 1m12s past the agent's start.
function recordRows(on: On) {
  const clock = mock.clock(on, { now: START + 72_000 })
  const bodies: (string | undefined)[] = []
  on('ui.render', { component: 'AgentRow' }, (_, e, next) => {
    bodies.push(e.props.body)
    return next(e)
  })
  return { clock, bodies, last: () => bodies.at(-1)?.replace(/\x1b\[[0-9;]*m/g, '') }
}

const mountRow = ($: Engine, surface: 'terminal' | 'desktop' = 'terminal') =>
  $.ui.mount({ plugin: 'kit', surface, component: 'AgentRow', props: ROW })

describe('subagent rows', () => {
  for (const surface of ['terminal', 'desktop'] as const) {
    test(`a subagent's tool call is the end of its row while it runs on ${surface}`, async ($, on) => {
      const rows = recordRows(on)
      let redraw = async (): Promise<void> => {}
      let during: string | undefined
      on('tool.call', async () => {
        await rows.clock.settle()
        await redraw()
        during = rows.last()
        return { result: RAN }
      })

      const ui = await mountRow($, surface)
      redraw = () => ui.redraw()
      expect(rows.last()).toBe(IDLE)

      const ran = await $.tool.call({ tool: 'Bash', command: 'go test ./...', agentId: 'a1' })
      await rows.clock.settle()
      await ui.redraw()

      expect(ran.result).toEqual(RAN)
      expect(during).toBe('Explore  running  1m12s  Bash go test ./...')
      expect(rows.last()).toBe(IDLE)
      await ui.unmount()
    })
  }

  test('a denied call leaves the row once it returns', async ($, on) => {
    const rows = recordRows(on)
    on('tool.call', () => ({ deny: 'no' }))

    const ui = await mountRow($)
    await $.tool.call({ tool: 'Bash', command: 'rm -rf build', agentId: 'a1' })
    await ui.redraw()

    expect(rows.last()).toBe(IDLE)
    await ui.unmount()
  })

  test("another agent's tool call leaves this row alone", async ($, on) => {
    const rows = recordRows(on)
    let redraw = async (): Promise<void> => {}
    let during: string | undefined
    on('tool.call', async () => {
      await redraw()
      during = rows.last()
      return { result: RAN }
    })

    const ui = await mountRow($)
    redraw = () => ui.redraw()
    await $.tool.call({ tool: 'Bash', command: 'ls', agentId: 'a2' })

    expect(during).toBe(IDLE)
    await ui.unmount()
  })

  test('switched off, the row keeps the engine body', { options: { subagent_rows: false } }, async ($, on) => {
    const rows = recordRows(on)

    const ui = await mountRow($)

    expect(rows.bodies).toEqual([undefined])
    await ui.unmount()
  })
})
