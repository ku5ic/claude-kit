import type { Register } from 'claude-code'
import { describeTool, formatRow } from './row.js'

export const register: Register = (on, options) => {
  if (options.subagent_rows === false) return

  const lastTool = new Map<string, string>()

  on('tool.call', ($, e, next) => {
    if (e.agentId !== undefined) {
      lastTool.set(e.agentId, describeTool(e.tool, e))
      $.ui.invalidate('ui.render')
    }
    return next(e)
  }).catch(($, e, next) => next(e))

  on('ui.render', { component: 'AgentRow' }, async ($, e, next) =>
    next({ ...e, props: { ...e.props, body: formatRow(e.props, lastTool.get(e.props.id), await $.clock.now()) } }),
  )
}
