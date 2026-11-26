---
name: code-from-design
description: Map a design to the repo's components and tokens, with what's missing, states the design omits, and accessibility gaps, for /plan
argument-hint: "<design URL>"
allowed-tools:
  - Bash(git log *)
  - Bash(kit contrast *)
  - Bash(kit detect-stack)
  - Bash(kit scratch-dir *)
---

# /code-from-design

Read a design through the design tool's connector and map it to what the repo already has: the components and tokens that cover it, what's missing, the states it leaves out, and its accessibility gaps. The report feeds `/plan`. Read-only, writes no code.

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                              | Read                                                             |
| ----------------------------------- | ---------------------------------------------------------------- |
| Step 2, reading the design          | [reference/design-connectors.md](reference/design-connectors.md) |
| Step 4, mapping elements and values | [reference/mapping.md](reference/mapping.md)                     |
| Step 5, states and accessibility    | [reference/states-and-gaps.md](reference/states-and-gaps.md)     |

## Procedure

1. Inputs. No URL, ask. The service comes from the URL's domain and its connector by `skills/kit-rules/reference/tooling.md` section 4. None, ask for an export or a screenshot and work from that, saying so.
2. Read the design per [reference/design-connectors.md](reference/design-connectors.md), the outline first, then the chosen nodes. Raw responses go to the report's sibling folder.
3. Inventory the repo, its components, stories, token sources (the audit-tokens skill's `reference/token-files.md`), and design-to-code mapping files, with the stack's pattern skill loaded.
4. Map each element and value per [reference/mapping.md](reference/mapping.md).
5. States, breakpoints, and accessibility per [reference/states-and-gaps.md](reference/states-and-gaps.md). Contrast from `kit contrast` on resolved values, criteria by the wcag-audit skill.

## Report

Load the report-format skill and use its format, written to the path `kit scratch-dir from-design <design-slug>` prints, with these narrowings:

- The TL;DR's first bullet is the coverage in place of the health word: `<covered> covered, <extend> to extend, <compose> to compose, <new> new, <omitted> states omitted`.
- A Mapping table with the design element, its result, the repo component or token, and the files it would touch.
- A Missing tokens table with the design value and the nearest existing token.
- The last section lists candidate `/plan` steps, one per `extend`, `compose`, or `new` item.

## Reply

The report path and the coverage line.

## Stop

Read-only on the repo and in the design tool. No code, no comments on the design.

Arguments: `$ARGUMENTS`
