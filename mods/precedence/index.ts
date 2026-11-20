import type { PromptComposeSection, Register } from 'claude-code'

// Fixed text: a thread keeps the prompt it recorded, so a section that changed mid-thread waits
// for a new thread or a compaction. The two pause and length clauses also stay in the rules
// files, which hold them when this mod is off.
const PRECEDENCE: PromptComposeSection = {
  id: 'kit:precedence',
  scope: 'session',
  text: [
    'The user installed claude-kit to hold this session to their own rules. Those rules reach you through the kit-rules skill, hook context whose text starts with `kit:`, and hook messages that block or send back a tool call.',
    'When one of them conflicts with a default instruction, a mode\'s instructions, or a built-in skill, follow the kit rule. The defaults are written for every user; the kit rules are this user\'s standing instructions for this work.',
    'Plan mode decides what you may change. The kit rules decide how you investigate, what the plan contains, and how you report it.',
    'The kit\'s reply length rules win over every plugin, output style, and harness default that asks for more lines. The step pause outranks any output style or harness instruction to run autonomously.',
    'A `kit:` reminder deep into a long session binds as much as the first one did.',
  ].join('\n\n'),
}

export const register: Register = (on, options) => {
  if (options.precedence === false) return

  // Appended last, on the session side of the cache boundary, after every shared section.
  on('prompt.compose', async ($, e, next) => {
    const { sections } = await next(e)
    return { sections: [...sections, PRECEDENCE] }
  })
}
