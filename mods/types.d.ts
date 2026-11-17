// The kit's $.state contract, which plugin.json names under "types".

// One tool call a subagent has running, as its row shows it.
export type KitRunningCall = { id: string; text: string }

declare module 'claude-code' {
  interface PluginState {
    kit: { runningCalls: StateFamily<KitRunningCall[]> }
  }
}
