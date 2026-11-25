import type { EngineInterface, Register } from 'claude-code'
import { STATS_COMMAND } from '../stats/index.js'

// Downloads the binary when an install or update left none: kit version is a CLI call, which
// bin/kit fetches for. Without it, hooks fail open and the SessionStart hook says why, so a
// failure here needs no word of its own.
async function fetchBinary($: EngineInterface): Promise<void> {
  try {
    const env = await $.process.hookEnv()
    await $.process.run([`${env.CLAUDE_PLUGIN_ROOT}/bin/kit`, 'version'], { env })
  } catch {
    return
  }
}

// The kit's one session.start hook, since a second one fails the load. Where no SessionStart hook
// runs, a /reload-plugins after an update, this one does, for the new version's module.
export const register: Register = (on, options) => {
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    // After the start, so a slow download never holds it up.
    $.clock.after(0, () => fetchBinary($))
    // A command is registered for the session, so on each start.
    if (options.kit_stats !== false) {
      await $.command.register({
        name: STATS_COMMAND,
        description: 'Show kit stats: blocks, shadow blocks, and skips per rule, and the claims each checker flagged.',
        argumentHint: '[--since days] [--rule name] [--unused]',
      })
    }
    return started
  })
}
