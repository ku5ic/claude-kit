// The kit's $.state contract, which plugin.json names under "types".

// One tool call a subagent has running, as its row shows it.
export type KitRunningCall = { id: string; text: string }

// Whether prompt.compose ran in this turn, and whether the last answered turn went without it.
export type KitPrecedence = { composed: boolean; armed: boolean }

declare module 'claude-code' {
  interface PluginState {
    kit: { runningCalls: StateFamily<KitRunningCall[]>; precedence: KitPrecedence }
  }
}
