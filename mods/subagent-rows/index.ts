import type { Register } from 'claude-code'
import { countDescendants, describeTool, formatRow } from './row.js'

export const register: Register = (on, options) => {
  if (options.subagent_rows === false) return

  const running = new Map<string, string>()

  // A tool.call hook that may match a call turns off its move to the background and its live output.
  // The agentId matcher leaves the main loop's calls out of that.
  on('tool.call', { agentId: /./ }, async ($, e, next) => {
    const { agentId } = e
    if (agentId === undefined) return next(e)
    const call = describeTool(e.tool, e)
    if (running.get(agentId) !== call) $.ui.invalidate('ui.render')
    running.set(agentId, call)
    try {
      return await next(e)
    } finally {
      if (running.get(agentId) === call) {
        running.delete(agentId)
        $.ui.invalidate('ui.render')
      }
    }
  }).catch(($, e, next) => next(e))

  // The engine asks every row again as an agent comes or goes, so the count stays current.
  on('ui.render', { component: 'AgentRow' }, async ($, e, next) => {
    const { props } = e
    const [agents, now] = await Promise.all([$.agent.list(), $.clock.now()])
    return next({ ...e, props: { ...props, body: formatRow(props, running.get(props.id), now, countDescendants(props.id, agents)) } })
  })
}
