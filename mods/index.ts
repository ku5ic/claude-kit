import type { Register } from 'claude-code'
import { register as precedence } from './precedence/index.js'
import { start as startSession } from './session/index.js'
import { register as stats, start as startStats } from './stats/index.js'
import { register as subagentRows } from './subagent-rows/index.js'

// The one module hooks/hooks.json loads, since `modules` takes a single path. Each mod is a
// folder here whose register this calls, switched off by its own userConfig key in plugin.json,
// session excepted. A second session.start hook fails the load, so this holds the only one,
// and a mod's work at session start is its exported start, called from it.

export const register: Register = (on, options) => {
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    startSession($)
    await startStats($, options)
    return started
  })
  precedence(on, options)
  stats(on, options)
  subagentRows(on, options)
}
