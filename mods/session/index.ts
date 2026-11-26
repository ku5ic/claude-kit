import type { EngineInterface } from 'claude-code'

// Downloads the binary when an install or update left none: kit version is a CLI call, which
// bin/kit fetches for. Without it, hooks fail open and the SessionStart hook says why, so a
// failure here needs no word of its own.
async function fetchBinary($: EngineInterface): Promise<void> {
  try {
    const env = await $.process.hookEnv()
    await $.process.run([`${env.CLAUDE_PLUGIN_ROOT}/bin/kit`, 'version'], { env })
  } catch {
    return
  }
}

// Where no SessionStart hook runs, a /reload-plugins after an update, this does, for the new
// version's module. After the start, so a slow download never holds it up.
export function start($: EngineInterface): void {
  $.clock.after(0, () => fetchBinary($))
}
