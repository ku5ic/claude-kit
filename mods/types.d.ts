// The kit's $.state contract, which plugin.json names under "types".

// One tool call a subagent has running, as its row shows it.
export type KitRunningCall = { id: string; text: string }

// Whether the precedence section reached the model, and the hash of the first message its text
// last went out after.
export type KitPrecedence = { composed?: true; armed?: true; head?: number }

declare module 'claude-code' {
  interface PluginState {
    kit: { runningCalls: StateFamily<KitRunningCall[]>; precedence: KitPrecedence }
  }
}
