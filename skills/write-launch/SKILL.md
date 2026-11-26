---
name: write-launch
description: Launch copy for a project you built, from its README and code. An announcement post, landing page copy, or the facts for a Show HN you write yourself
argument-hint: "<post | landing | show-hn> [what's launching: the project, a release, or a feature]"
allowed-tools:
  - Bash(git status *)
  - Bash(git log *)
  - Bash(git describe *)
  - Bash(git branch --show-current)
  - Bash(git rev-parse *)
  - Bash(kit slop *)
---

# /write-launch

Launch copy that says what the project is, who it's for, and one thing it does that a reader can check, backed by the project's own README and code.

<!-- chat: only
In claude.ai chat, the arguments are the user's message and the material is what they pasted. Nothing can be run: if the README, manifest, or commit log this needs isn't in the conversation, ask for it.
-->

Loading this skill isn't enough. Before the step a row names, read its file.

| Before                   | Read                                                           |
| ------------------------ | -------------------------------------------------------------- |
| Step 1                   | `skills/slop-patterns/SKILL.md` and the rows it names for docs |
| Writing any of the kinds | [reference/launch-practice.md](reference/launch-practice.md)   |
| Copy that goes into a UI | `skills/slop-patterns/reference/design.md`                     |

## Procedure

<!-- chat: skip -->

1. Kind: the first word of $ARGUMENTS, `post`, `landing`, or `show-hn`. Missing, ask with AskUserQuestion, `post` first. The rest names what's launching; blank means the project itself.

<!-- chat: end -->

<!-- chat: only
1. Kind: `post`, `landing`, or `show-hn`, from the user's message. Missing, ask, `post` first. Blank on what's launching means the project itself.
-->

2. Facts. Read the README, the manifest (name, version, license), and the docs it links.

<!-- chat: skip -->

For a release, read the commits since the last tag: `git describe --tags --abbrev=0`, then `git log --oneline --no-merges <tag>..HEAD`. With no tag, `git log --oneline --no-merges` over the full history.

<!-- chat: end -->

Every line of the output traces to these or to the user's message (`skills/kit-rules/reference/evidence.md` section 1). A user count, benchmark, quote, or comparison nothing backs stays out.

3. Pitch: one sentence on what it is, who it's for, and what it replaces or saves them. Can't write it from the facts, ask.
4. Proof: one thing a reader can check. A command and its real output, when the project defines the command and it changes nothing; otherwise a number the repo shows. Never invent output.
5. Write the kind in its shape below.

<!-- chat: skip -->

6. Scan: `kit slop <path>`. Rewrite each hit by slop-patterns.

## Output file

Write to the path `kit scratch-dir launch <kind>-<slug>` prints.

<!-- chat: end -->

`post`:

```markdown
# <Name and what it does, 60 characters or less>

<The pitch, then the proof. Two short paragraphs.>

## <What it does: the three things a reader would search for, each with how>

## Try it

<Install, one real command, its real output.>

## What it doesn't do

<Known limits, from the README or the user.>

<One link: the repo or the docs.>
```

`landing`, copy only, one line per slot:

```text
Page title: <60 characters or less, keywords first>
Headline: <the offering, stated plainly>
Subhead: <the pitch>
Primary action: <the install command or the try link>
Proof: <the command and its output, or the number>
What it does: <three items, each with how>
```

`show-hn`, notes the user writes the post from:

```markdown
# Show HN notes: <name>

Write the title and text yourself: HN doesn't accept generated or AI-edited text.

Link: <the repo or a demo, never a landing page or post>
Try it: <the shortest path, with no sign-up>
What it is: <the pitch>
Why you built it: <from the user; ask if missing>
What's different: <one checkable difference>
Proof: <the command and output, or the number>
What's rough: <known limits>
Title: starts with "Show HN"; no uppercase emphasis, exclamation points, praise, or gratuitous number
```

## Rules

- No superlatives. A competitor gets named only when the user names it and the difference is checkable.
- A fact the sources don't back gets a question, never a filler.
- Copy only. The page's look comes from the project's own design tokens.
- Never post, submit, or publish anything.

<!-- chat: skip -->

Arguments: `$ARGUMENTS`

<!-- chat: end -->
