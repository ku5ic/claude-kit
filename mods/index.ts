import type { Register } from 'claude-code'
import { register as subagentRows } from './subagent-rows/index.js'

export const register: Register = (on, options) => {
  subagentRows(on, options)
}
