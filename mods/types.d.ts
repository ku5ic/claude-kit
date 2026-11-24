// The kit's $.state contract, which plugin.json names under "types".

// One tool call a subagent has running, as its row shows it.
export type KitRunningCall = { id: string; text: string }

// Whether prompt.compose ran since the last answered turn, and whether that turn went without it,
// absent until a turn is answered.
export type KitPrecedence = { composed: boolean; armed?: boolean }

declare module 'claude-code' {
  interface PluginState {
    kit: { runningCalls: StateFamily<KitRunningCall[]>; precedence: KitPrecedence }
  }
}
