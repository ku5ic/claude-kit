import type { On } from 'claude-code'
import { describe, expect, test, type Engine } from 'claude-code/testing'

const TABLE = 'rule         blocks  shadow  disabled  skipped  sessions\nplan-critic  1       0       0         1        1\n'

// Stands in for the engine: records each command registered and each process run, and
// answers the run with result.
function host(on: On, result: { exitCode: number; stdout: string; stderr: string }) {
  const registered: string[] = []
  const runs: { argv: readonly string[]; env?: Record<string, string> }[] = []
  on('command.register', (_, e) => {
    registered.push(e.name)
    return { value: { command: e.name } }
  })
  on('session.start', (_, e) => ({ cwd: e.cwd }))
  on('process.hookEnv', () => ({ value: { CLAUDE_PLUGIN_ROOT: '/plugin' } }))
  on('process.run', (_, e) => {
    runs.push({ argv: e.argv, env: e.init?.env })
    return { value: result }
  })
  return { registered, runs }
}

const start = ($: Engine) => $.session.start({ cwd: '/repo', surface: 'terminal', isInteractive: true })

describe('kit stats', () => {
  test('/kit-stats prints the table kit stats writes, run by the plugin root', async ($, on) => {
    const h = host(on, { exitCode: 0, stdout: TABLE, stderr: '' })
    await start($)
    expect(h.registered).toEqual(['kit-stats'])
    const out = await $.command.run({ command: 'kit-stats', args: '--since 7  --rule plan-critic' })
    expect(out.text).toBe(TABLE.trimEnd())
    expect(h.runs).toEqual([
      { argv: ['/plugin/bin/kit', 'stats', '--since', '7', '--rule', 'plan-critic'], env: { CLAUDE_PLUGIN_ROOT: '/plugin' } },
    ])
  })

  test('a failed run says so, with its exit code and what it wrote to stderr', async ($, on) => {
    host(on, { exitCode: 2, stdout: '', stderr: 'kit stats: unknown flag --sinse\n' })
    await start($)
    const out = await $.command.run({ command: 'kit-stats', args: '--sinse 7' })
    expect(out.text).toBe('kit stats exited 2: kit stats: unknown flag --sinse')
  })

  test('switched off, no command is registered', { options: { kit_stats: false } }, async ($, on) => {
    const h = host(on, { exitCode: 0, stdout: TABLE, stderr: '' })
    await start($)
    expect(h.registered).toEqual([])
  })
})
