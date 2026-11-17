import type { AgentInfo, AgentStatus } from 'claude-code'

const SEP = '  '

const STATUS_SGR: Record<AgentStatus, string> = {
  running: '32',
  pending: '33',
  waiting: '33',
  failed: '31',
  killed: '31',
  idle: '2',
  completed: '2',
}

const ENDED: ReadonlySet<AgentStatus> = new Set(['completed', 'failed', 'killed'])

// The input field that says what a tool call is about, in the order tried.
const ARG_KEYS = ['command', 'file_path', 'pattern', 'url', 'query', 'description', 'skill'] as const

export function formatRow(agent: AgentInfo, lastTool: string | undefined, now: number): string {
  const ended = ENDED.has(agent.status)
  const where = agent.status === 'running' && lastTool !== undefined ? lastTool : firstLine(agent.label)
  return [
    printable(agent.name ?? agent.type),
    `\x1b[${STATUS_SGR[agent.status] ?? '0'}m${agent.status}\x1b[0m`,
    printable(modelAndEffort(agent)),
    contextShare(agent),
    ended ? '' : duration(now - agent.startTime),
    printable(where),
  ]
    .filter(field => field !== '')
    .join(SEP)
}

export function describeTool(tool: string, input: Readonly<Record<string, unknown>>): string {
  for (const key of ARG_KEYS) {
    const value = input[key]
    if (typeof value !== 'string') continue
    const arg = printable(key === 'file_path' ? (value.split(/[\\/]/).filter(Boolean).pop() ?? '') : firstLine(value))
    if (arg !== '') return `${tool} ${arg}`
  }
  return tool
}

function modelAndEffort(agent: AgentInfo): string {
  const model = (agent.lastRequest?.model ?? agent.model)?.replace(/^claude-/, '')
  const effort = agent.lastRequest ? agent.lastRequest.effort : agent.effort
  const effortText = effort === undefined ? undefined : typeof effort === 'number' ? `effort:${effort}` : effort
  return [model, effortText].filter(part => part !== undefined).join(' ')
}

function contextShare(agent: AgentInfo): string {
  const size = agent.contextWindowSize
  return size ? `${Math.floor((agent.tokenCount * 100) / size)}%` : ''
}

function duration(ms: number): string {
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m${seconds % 60}s`
  return `${Math.floor(minutes / 60)}h${minutes % 60}m`
}

// body draws SGR and OSC 8 escapes, so model-written text loses its control and bidi characters.
function printable(text: string): string {
  return text.replace(/[\x00-\x1f\x7f-\x9f\u202a-\u202e\u2066-\u2069]/g, '')
}

function firstLine(text: string): string {
  const start = text.trimStart()
  const end = start.indexOf('\n')
  return (end === -1 ? start : start.slice(0, end)).trim()
}
