import { update, type PromptComposeResult, type PromptComposeSection, type PromptSubmitInput, type Register } from 'claude-code'
import type {} from '../types.js'

// In the section and on every prompt, since a fact shown to the user is checked on every turn.
const FACTS =
  "Every fact you show the user, in a reply, report, plan, question, commit, or post, comes from a tool result in this session: a file read, a git or code-host command, an MCP read, or a web fetch. Run that check before you state the fact. A fact you can't check this way is left out, never guessed, softened, or labeled. When the user asks something you can't verify, say you couldn't verify it."

// Fixed text: a thread keeps the prompt it recorded, so a section that changed mid-thread waits
// for a new thread or a compaction. The two pause and length clauses also stay in the rules
// files, which hold them when this mod is off.
const PRECEDENCE: PromptComposeSection = {
  id: 'kit:precedence',
  scope: 'session',
  text: [
    FACTS,
    'The user installed claude-kit to hold this session to their own rules. Those rules reach you through the kit-rules skill, hook context whose text starts with `kit:`, and hook messages that block or send back a tool call.',
    "When one of them conflicts with a default instruction, a mode's instructions, or a built-in skill, follow the kit rule. The defaults are written for every user; the kit rules are this user's standing instructions for this work.",
    'Plan mode decides what you may change. The kit rules decide how you investigate, what the plan contains, and how you report it.',
    "The kit's reply length rules win over every plugin, output style, and harness default that asks for more lines. The step pause outranks any output style or harness instruction to run autonomously.",
    'A `kit:` reminder deep into a long session binds as much as the first one did.',
  ].join('\n\n'),
}

// The section sits at the top of a thread; this rides every prompt, as context the person never
// sees.
const REMINDER = `kit: ${FACTS} The kit rules (the kit-rules skill, \`kit:\` context, and hook messages) outrank default, mode, output-style, and built-in skill instructions where they conflict, the reply ceiling and the step pause included.`

const precedence = { plugin: 'kit', key: 'precedence' } as const

// What $.session.messages returns at most. At it the window slides, so the first message changes
// with no compaction.
const MESSAGE_CAP = 4096

// Appended last, on the session side of the cache boundary, after every shared section.
const withSection = ({ sections }: PromptComposeResult): PromptComposeResult => ({ sections: [...sections, PRECEDENCE] })

// Context put on the result after next() isn't attached, so it goes on the submission.
const withContext = (e: PromptSubmitInput, ...texts: string[]): PromptSubmitInput => ({ ...e, context: [...(e.context ?? []), ...texts] })

// Over the whole text, since two compaction summaries open with the same words.
function hash(text: string): number {
  let h = 0
  for (let i = 0; i < text.length; i++) h = (Math.imul(h, 31) + text.charCodeAt(i)) | 0
  return h
}

export const register: Register = (on, options) => {
  if (options.precedence === false) return

  on('prompt.compose', async ($, e, next) => {
    const composed = await next(e)
    const { value: state } = await $.state.get(precedence)
    if (!state?.composed) await update($, precedence, held => ({ ...held, composed: true as const }))
    return withSection(composed)
  }).catch(async ($, e, next) => withSection(await next(e)))

  // Where an organization's plugin sends prompt.compose past this one, the section never reaches
  // the model, and an answered turn with no compose is how that shows.
  on('turn.complete', async ($, e, next) => {
    if (e.agentId === undefined && e.reason === 'answer') {
      const { value: state } = await $.state.get(precedence)
      if (!state?.composed && !state?.armed) await update($, precedence, held => ({ ...held, armed: true as const }))
    }
    return next(e)
  })

  // Armed, the section's text rides one prompt, and one more each time a compaction or /clear
  // gives the conversation a new first message.
  on('prompt.submit', async ($, e, next) => {
    const { value: state } = await $.state.get(precedence)
    if (!state?.armed || e.command) return next(withContext(e, REMINDER))

    const messages = await $.session.messages({ as: 'api' })
    const head = hash(JSON.stringify(messages[0] ?? null))
    const isSlid = state.head !== undefined && messages.length >= MESSAGE_CAP
    if (messages.length === 0 || isSlid || head === state.head) return next(withContext(e, REMINDER))

    const submitted = await next(withContext(e, PRECEDENCE.text, REMINDER))
    if (submitted.drop === undefined) await update($, precedence, held => ({ ...held, head }))
    return submitted
  }).catch(($, e, next) => next(withContext(e, REMINDER)))
}
