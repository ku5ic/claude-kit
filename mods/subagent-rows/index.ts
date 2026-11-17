import { update, type Register } from 'claude-code'
import type { KitRunningCall } from '../types.js'
import { countDescendants, describeTool, formatRow } from './row.js'

// The calls each agent has running, oldest first, in $.state so a hot reload keeps them.
const runningCalls = { plugin: 'kit', key: 'runningCalls' } as const

export const register: Register = (on, options) => {
  if (options.subagent_rows === false) return

  // A tool.call hook that may match a call turns off its move to the background and its live output.
  // The agentId matcher leaves the main loop's calls out of that. The catch stands in for a hook that
  // fails: without it the hook is skipped, and with it next's settled result is replayed, not rerun.
  on('tool.call', { agentId: /./ }, async ($, e, next) => {
    const { agentId } = e
    if (agentId === undefined) return next(e)
    const ref = { ...runningCalls, id: agentId }
    const call: KitRunningCall = { id: e.tool_use_id, text: describeTool(e.tool, e) }
    await update($, ref, calls => [...(calls ?? []), call])
    try {
      return await next(e)
    } finally {
      await update($, ref, calls => (calls ?? []).filter(one => one.id !== call.id))
    }
  }).catch(($, e, next) => next(e))

  // The engine asks every row again as an agent comes or goes, so the count stays current.
  on('ui.render', { component: 'AgentRow' }, async ($, e, next) => {
    const { props } = e
    // Reading the calls while drawing redraws the row each time they change.
    const [agents, now, calls] = await Promise.all([$.agent.list(), $.clock.now(), $.state.get({ ...runningCalls, id: props.id })])
    const lastTool = calls.value?.at(-1)?.text
    return next({ ...e, props: { ...props, body: formatRow(props, lastTool, now, countDescendants(props.id, agents)) } })
  })
}
