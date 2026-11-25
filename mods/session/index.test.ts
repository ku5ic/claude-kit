import type { On } from 'claude-code'
import { describe, expect, mock, test, type Engine } from 'claude-code/testing'

// Stands in for the engine: records each command registered and each process run, which run
// answers.
function host(on: On, run: () => { exitCode: number }) {
  const clock = mock.clock(on)
  const registered: string[] = []
  const runs: (readonly string[])[] = []
  on('command.register', (_, e) => {
    registered.push(e.name)
    return { value: { command: e.name } }
  })
  on('session.start', (_, e) => ({ cwd: e.cwd }))
  on('process.hookEnv', () => ({ value: { CLAUDE_PLUGIN_ROOT: '/plugin' } }))
  on('process.run', (_, e) => {
    runs.push(e.argv)
    return { value: { ...run(), stdout: '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })
  return { clock, registered, runs }
}

const start = ($: Engine) => $.session.start({ cwd: '/repo', surface: 'terminal', isInteractive: true })

const VERSION = ['/plugin/bin/kit', 'version']

describe('session start', () => {
  test('kit version runs from the plugin root, after the start, which does not wait for it', async ($, on) => {
    const h = host(on, () => ({ exitCode: 0 }))
    expect(await start($)).toEqual({ cwd: '/repo' })
    expect(h.runs).toEqual([])
    await h.clock.settle()
    expect(h.runs).toEqual([VERSION])
    expect(h.registered).toEqual(['kit-stats'])
  })

  test('each start runs it, as after a reload', async ($, on) => {
    const h = host(on, () => ({ exitCode: 0 }))
    await start($)
    await h.clock.settle()
    await start($)
    await h.clock.settle()
    expect(h.runs).toEqual([VERSION, VERSION])
  })

  test('with kit_stats off it still runs, and no command is registered', { options: { kit_stats: false } }, async ($, on) => {
    const h = host(on, () => ({ exitCode: 0 }))
    await start($)
    await h.clock.settle()
    expect(h.runs).toEqual([VERSION])
    expect(h.registered).toEqual([])
  })

  test('a run that fails or cannot start leaves the session and the command alone', async ($, on) => {
    const h = host(on, () => {
      throw new Error('no such file')
    })
    expect(await start($)).toEqual({ cwd: '/repo' })
    await h.clock.settle()
    expect(h.runs).toEqual([VERSION])
    expect(h.registered).toEqual(['kit-stats'])
  })
})
