import type { AgentInfo } from 'claude-code'
import { describe, expect, test } from 'claude-code/testing'
import { describeTool, formatRow } from './row.js'

const NOW = 1_000_000

function agent(overrides: Partial<AgentInfo> = {}): AgentInfo {
  return {
    id: 'a1',
    description: 'find the row hook',
    type: 'Explore',
    status: 'running',
    kind: 'local_agent',
    taskStatus: 'running',
    label: 'find the row hook',
    startTime: NOW - 72_000,
    cwd: '/repo',
    tokenCount: 0,
    tokenSamples: [],
    hasRow: true,
    ...overrides,
  }
}

const plain = (row: string): string => row.replace(/\x1b\[[0-9;]*m/g, '')

describe('formatRow', () => {
  test('leads with the name, else the type', () => {
    expect(plain(formatRow(agent({ name: 'scout' }), undefined, NOW))).toMatch(/^scout  running  /)
    expect(plain(formatRow(agent(), undefined, NOW))).toMatch(/^Explore  running  /)
  })

  test('colours the status by where the loop stands', () => {
    expect(formatRow(agent({ status: 'running' }), undefined, NOW)).toContain('\x1b[32mrunning\x1b[0m')
    expect(formatRow(agent({ status: 'waiting' }), undefined, NOW)).toContain('\x1b[33mwaiting\x1b[0m')
    expect(formatRow(agent({ status: 'failed' }), undefined, NOW)).toContain('\x1b[31mfailed\x1b[0m')
    expect(formatRow(agent({ status: 'completed' }), undefined, NOW)).toContain('\x1b[2mcompleted\x1b[0m')
  })

  test('shows the model that answered, else the one set, without the claude- prefix', () => {
    const answered = agent({ model: 'claude-opus-5-5', lastRequest: { at: NOW, model: 'claude-haiku-5-5' } })
    expect(plain(formatRow(answered, undefined, NOW))).toContain('  haiku-5-5  ')
    expect(plain(formatRow(agent({ model: 'claude-opus-5-5' }), undefined, NOW))).toContain('  opus-5-5  ')
    expect(plain(formatRow(agent(), undefined, NOW))).not.toContain('opus')
  })

  test('shows the effort sent, else the one set, a number as effort:<n>', () => {
    const sent = agent({ effort: 'max', lastRequest: { at: NOW, model: 'claude-opus-5-5', effort: 'high' } })
    expect(plain(formatRow(sent, undefined, NOW))).toContain('  opus-5-5 high  ')
    expect(plain(formatRow(agent({ effort: 'low' }), undefined, NOW))).toContain('  low  ')
    expect(plain(formatRow(agent({ effort: 12000 }), undefined, NOW))).toContain('  effort:12000  ')
  })

  test('shows the share of the context window used, only with a window size', () => {
    expect(plain(formatRow(agent({ tokenCount: 64_000, contextWindowSize: 200_000 }), undefined, NOW))).toContain('  32%  ')
    expect(plain(formatRow(agent({ tokenCount: 64_000 }), undefined, NOW))).not.toContain('%')
  })

  test('shows elapsed time while the agent lives, none once it ended', () => {
    expect(plain(formatRow(agent({ startTime: NOW - 45_000 }), undefined, NOW))).toContain('  45s  ')
    expect(plain(formatRow(agent({ startTime: NOW - 72_000 }), undefined, NOW))).toContain('  1m12s  ')
    expect(plain(formatRow(agent({ startTime: NOW - 3_780_000 }), undefined, NOW))).toContain('  1h3m  ')
    expect(plain(formatRow(agent({ status: 'completed' }), undefined, NOW))).not.toContain('1m12s')
  })

  test('ends with the last tool while running, else the first line of the label', () => {
    expect(plain(formatRow(agent(), 'Bash go test ./...', NOW))).toMatch(/  Bash go test \.\/\.\.\.$/)
    expect(plain(formatRow(agent({ label: 'Searching hooks\nthen reading' }), undefined, NOW))).toMatch(/  Searching hooks$/)
    expect(plain(formatRow(agent({ status: 'completed', label: 'Done' }), 'Bash ls', NOW))).toMatch(/  Done$/)
  })

  test('drops control characters from what agents and models wrote', () => {
    const hostile = agent({ name: 'a\x1b[2Jb', label: 'x\x1b]8;;https://evil\x07y', model: 'claude-\x1b[31mopus' })
    const row = formatRow(hostile, undefined, NOW).replace(/\x1b\[\d+mrunning\x1b\[0m/, 'running')
    expect(row).not.toMatch(/[\x00-\x1f\x7f]/)
    expect(formatRow(agent(), 'Bash ls\x1b[2J', NOW)).not.toContain('\x1b[2J')
  })
})

describe('describeTool', () => {
  test('names the tool with its main argument', () => {
    expect(describeTool('Bash', { command: 'go test ./...\necho done' })).toBe('Bash go test ./...')
    expect(describeTool('Read', { file_path: '/repo/go/internal/status/statusline.go' })).toBe('Read statusline.go')
    expect(describeTool('Grep', { pattern: 'AgentRow' })).toBe('Grep AgentRow')
    expect(describeTool('WebFetch', { url: 'https://example.com' })).toBe('WebFetch https://example.com')
    expect(describeTool('Agent', { description: 'Fact-check the report' })).toBe('Agent Fact-check the report')
  })

  test('names the tool alone when no argument is known', () => {
    expect(describeTool('TodoWrite', { todos: [] })).toBe('TodoWrite')
    expect(describeTool('Bash', { command: '' })).toBe('Bash')
  })

  test('drops control characters from the argument', () => {
    expect(describeTool('Bash', { command: 'ls \x1b]8;;https://evil\x07x' })).toBe('Bash ls ]8;;https://evilx')
  })
})
