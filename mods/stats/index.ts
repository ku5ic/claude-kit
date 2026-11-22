import type { Register } from 'claude-code'

const COMMAND = 'kit-stats'

export const register: Register = (on, options) => {
  if (options.kit_stats === false) return

  // A command is registered for the session, so on each start; a reload runs this again.
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    await $.command.register({
      name: COMMAND,
      description: 'Show kit stats: blocks, shadow blocks, and skips per rule, and the claims each checker flagged.',
      argumentHint: '[--since days] [--rule name]',
    })
    return started
  })

  // The binary writes the table; the command prints it as written. kit stats parses its own
  // flags, and the argv runs with no shell, so what was typed after the name passes through.
  on('command.run', { command: COMMAND }, async ($, e) => {
    const env = await $.process.hookEnv()
    const args = e.args.split(/\s+/).filter(Boolean)
    try {
      const { exitCode, stdout, stderr } = await $.process.run([`${env.CLAUDE_PLUGIN_ROOT}/bin/kit`, 'stats', ...args], { env })
      return { text: exitCode === 0 ? stdout.trimEnd() : `kit stats exited ${exitCode}: ${stderr.trim()}` }
    } catch (err) {
      return { text: `kit stats didn't run: ${err instanceof Error ? err.message : String(err)}` }
    }
  }).catch(() => ({ text: "kit stats didn't run: the command hook failed or timed out" }))
}
