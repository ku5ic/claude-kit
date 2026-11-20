import type { Register } from 'claude-code'
import { register as precedence } from './precedence/index.js'
import { register as stats } from './stats/index.js'
import { register as subagentRows } from './subagent-rows/index.js'

// The one module hooks/hooks.json loads, since `modules` takes a single path. Each mod is a
// folder here whose register this calls, switched off by its own userConfig key in plugin.json.

export const register: Register = (on, options) => {
  precedence(on, options)
  stats(on, options)
  subagentRows(on, options)
}
