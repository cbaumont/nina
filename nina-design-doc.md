# Design Document: Nina — an AI Pair Programming Companion

**Status:** Draft v0.3
**Author:** Carlos
**Date:** July 2026

## 1. Summary

Nina is a CLI-based AI pair programming companion for people who want to *learn* to code, not just have code written for them. It follows classic pair programming: the human is the **driver** who types, and the AI is the **navigator** who directs, explains, and reviews. Nina can scaffold projects, generate boilerplate, and run or debug code, but substantive typing stays with the user, guided by explanations calibrated to their experience.

The core hypothesis is that actively writing code with expert guidance teaches better than passively consuming AI-generated code. Current tools are optimized for productivity, leaving pedagogy underserved.

## 2. Goals and Non-Goals

### Goals

The MVP lets a user start a guided coding session in a terminal beside their preferred editor, work through a task in small instructed steps, and receive explanations of *what* to do, *how* to do it, and *why*. MVP sessions use AI-generated learning projects only; own-project work is post-MVP (see 4.2). Users can configure their programming experience, stack familiarity, and how much Nina may type on their behalf. Nina can also run and debug code, review what the user typed, and treat mistakes as teaching moments.

### Non-Goals (for MVP)

Nina is not a productivity tool; if the user wants the task done fast, other tools exist. The MVP will not include an editor extension, a GUI, multi-user/classroom features, structured curricula with progress tracking, or gamification. It does not sandbox commands; it relies on explicit confirmation, matching the trust model of running code on the user's own machine.

## 3. Target Users

The primary audience is anyone learning a language, framework, or technology. Because that spans a wide range, experience configuration is first-class. Three representative personas:

**The newcomer.** Has never programmed. Needs fundamentals, very small steps, and heavy encouragement. Likely working through an AI-generated learning project.

**The switcher.** An experienced developer learning a new stack. Needs terse instructions, idiom-level explanations, and analogies to what they already know. Post-MVP, likely working in their own project; for MVP, working through a generated project in the target stack.

**The upskiller.** Knows the stack basics but wants depth in a specific area: testing, async, performance, or a new framework version. Wants best practices and trade-offs, not syntax.

## 4. User Experience

### 4.1 The Core Loop

A session follows a repeating rhythm modeled on driver/navigator pair programming:

1. **Orient.** The navigator explains the current goal in context: what comes next and why it belongs there.
2. **Instruct.** The navigator gives a concrete, right-sized instruction: which file to open and what to write, at a specificity matching the user's profile and dial.
3. **Drive.** The user types in their editor and signals completion with `/done`. Nina watches the file system, but file events are a hint, not the authority: automatic review only fires after the user is idle on relevant files and watch reliability is established. Explicit `/done` is always authoritative.
4. **Review.** The navigator reads the user's diff and evaluates it against the step's *goal*, not a reference solution. Any correct solution is accepted; valid alternatives are acknowledged with trade-offs. Incorrect code gets Socratic handling by default, with hint escalation controlled by the user's settings. When uncertain, the navigator says so and leans on Verify rather than guessing.
5. **Verify.** Where possible, the navigator runs the code or tests, feeds back the output, and teaches the user to interpret it. In learning projects, Nina scaffolds the test harness as ground truth unless writing tests is the learning goal.
6. **Advance.** Move to the next step, periodically zooming out to recap how the pieces fit together.

The loop is a default path, not a straitjacket. Users can interrupt, backtrack, or go off-script. The engine treats step state as resumable: interruptions are answered in place; backtracking reloads the relevant snapshot as the new baseline; off-script edits are noted but not reviewed unless asked. Task plan revisions are always announced.

The session transcript is a learning artifact. At the end, Nina can summarize what was built, concepts covered, and topics to revisit.

### 4.2 Session Types

**Learning project mode.** The user describes what they want to learn, Nina proposes 2–3 project ideas sized to a few sessions, then scaffolds the repository and begins the loop. Generated projects should feel real: small genuine applications, not exercises.

**Own-project mode (post-MVP).** The user points Nina at an existing repository and states a goal. Nina first explores structure, conventions, and dependencies, then narrates a guided tour before planning the work. This doubles as codebase onboarding, but is deliberately cut from MVP: arbitrary-repo exploration and plan derivation are a second product's worth of work.

Both modes converge on the same core loop; they differ only in how the task backlog originates.

### 4.3 Configuration: the Profile and the Dial

At first run (and adjustable any time with `/profile`), the user sets:

**General programming experience** — none / beginner / intermediate / professional. Controls how much foundational explanation accompanies instructions.

**Stack familiarity** — per language/framework in play, on the same scale. Controls idiom-level vs. syntax-level explanation and analogies to known stacks. Nina also *infers and updates* this over time: a lightweight model pass at step boundaries notes signals and proposes adjustments through `/profile` — no silent drift.

**The typing dial** — how much the AI is permitted to write, as an explicit, user-controlled setting:

| Level | AI may write | Intended for |
|---|---|---|
| 0 — Full manual | Nothing. Instructions only. | Purists; exam prep |
| 1 — Scaffold | Project setup, config files, package installs | MVP default |
| 2 — Boilerplate | Level 1 + repetitive/no-learning-value code (imports, fixtures, type stubs) | Users who want more AI assistance |
| 3 — Collaborative | Level 2 + AI may take the keyboard for stretches the user delegates ("you write the CSS, I'll focus on the logic") | Time-boxed sessions; upskillers |

Three principles govern the dial. First, *the dial is a ceiling, not a target*: even at level 3, the navigator leaves the user anything with learning value. Second, *learning value is per-user*: imports may be boilerplate for a switcher and curriculum for a newcomer. Third, *AI-written code is transparent*: whenever Nina writes code, it says so, explains it, and may quiz the user on it later.

A related setting controls **hint escalation speed** in review: how quickly Nina moves from a nudge to a precise diagnosis to showing the fix.

### 4.4 Interaction Design in a Terminal

The CLI must feel like a companion, not a REPL. Key affordances: a persistent status line (current task, step, dial level); markdown with syntax highlighting; slash commands (`/why`, `/stuck`, `/skip`, `/recap`, `/profile`, `/dial`, `/run`, `/done`); and streaming where messages are not gated by dial-safety screening. Nina never instruments or controls the editor; it observes workspace file changes, reads files for the active session, and modifies files only within the permitted dial level after announcing it.

### 4.5 First Run

The first ten minutes decide whether a learner returns. First run includes an environment self-check, profile setup in under two minutes, and one tiny end-to-end warm-up step before the real project starts. Nina owns getting to a working baseline, treating environment debugging as a teaching moment only when the user has the background to benefit.

## 5. Architecture

### 5.1 Overview

```
┌─────────────────────────────┐
│  CLI (TUI)                  │  MVP runtime TBD: Node.js/Ink or Python/Textual
│  - session loop & commands  │
│  - rendering, status line   │
└──────────┬──────────────────┘
           │
┌──────────┴──────────────────┐
│  Session Engine             │
│  - task plan & step state   │
│  - profile & dial policy    │
│  - prompt assembly          │
│  - transcript & summaries   │
└───┬──────────┬──────────┬───┘
    │          │          │
┌───┴────┐ ┌───┴──────┐ ┌─┴───────────┐
│ LLM    │ │ Workspace│ │ Runner      │
│ client │ │ watcher  │ │ - run code  │
│(Claude │ │ - fs     │ │ - tests     │
│  API)  │ │   events │ │ - capture   │
│        │ │ - diffs  │ │   output    │
└────────┘ └──────────┘ └─────────────┘
```

### 5.2 Components

**Session Engine.** The heart of the system. Maintains the task plan, step state machine (orient → instruct → drive → review → verify → advance), and user profile. The engine, not the model's goodwill, enforces the typing dial: LLM file-write tool calls are filtered against the current policy, so a level-0 session cannot result in AI-written code.

The chat channel gets a similar guard. A fast-model pass screens outgoing navigator messages at dial levels 0–1 and regenerates messages that contain step-complete code. This is imperfect, but turns over-helping from a prompt-only risk into an engineered control. Prompt assembly injects the profile, dial policy, task plan, recent diffs, and pedagogical instructions into each model call.

**Workspace Watcher.** Uses native file-system events (chokidar / watchdog) plus git. Nina initializes or uses the repo's git to snapshot state at each step boundary, so "what did the user just write" is a clean diff. Snapshots live under hidden `refs/nina/*` refs created with git plumbing; Nina does not switch branches, create visible commits, or touch the user's history. File events feed the trigger policy in 4.1, with `/done` always authoritative.

**Runner.** Executes build/test/run commands with captured stdout/stderr fed back to both the user and the model. Commands are proposed by the model and confirmed by the user, with optional auto-approval for safe classes like the project's own test command. The Runner also owns environment health for learning projects: dependency installs, version checks, and the first-run self-check. MVP has no sandbox beyond confirmation.

**LLM Client.** Claude API via the official SDK, using tool use for structured actions (`write_file`, `read_file`, `run_command`, `update_plan`, `update_profile`). A strong model handles planning, review, and explanation; a fast model handles lightweight classification such as step completion, dial screening, and profile-inference signals. Session context is managed by summarizing older steps. With bring-your-own-key billing, define and track a target cost per typical session during beta.

**State & Persistence.** A `.nina/` directory holds the task plan, profile overrides, transcripts, and pointers to git snapshots, making sessions resumable and powering end-of-session summaries.

### 5.3 Key Design Decisions

**Git as the source of truth for "what the user did".** Snapshotting at step boundaries and diffing is simple, robust, editor-independent, and teaches git hygiene by osmosis. Hidden `refs/nina/*` snapshots keep Nina's state out of the user's visible history.

**Dial enforcement in the engine, not the prompt.** Prompt-only enforcement will leak. Making the dial a hard filter on tool calls turns the pedagogy constraint into an architectural guarantee: *it can't just do it for you*.

**Pedagogy as explicit prompt policy.** Teaching behavior — Socratic review, hint escalation, right-sized steps, and "why" alongside "what" — is encoded as versioned prompt files assembled from the profile, so it can be tuned rapidly against real sessions.

## 6. MVP Scope

The MVP is a single-user CLI. Runtime and packaging are an explicit open choice: Node.js/Ink via npm or Python/Textual via pipx. It supports one or two launch stacks well, proposed as Python and JavaScript/TypeScript. It includes learning-project mode, profile and dial, engine-level dial enforcement, low-dial message screening, the core loop, file watching, git snapshots, runner confirmation, environment self-check, session resume, and end-of-session summaries. Bring-your-own API keys keep billing out of scope initially.

Explicitly deferred: own-project mode (4.2), additional stacks, long-term learner progress models across projects, spaced-repetition review of past concepts, voice interaction, and any hosted/account infrastructure.

## 7. Success Criteria

MVP validation should answer four questions:

- Do users complete sessions instead of abandoning them mid-loop?
- Do they return within week one?
- Do they keep the dial low, or turn Nina into a code generator?
- Can they explain the code they wrote afterward?

A closed beta of 15–25 learners across the three personas, with transcripts reviewed by hand, will teach more than metrics alone. Transcript review should grade review accuracy and over-helping leakage.

## 8. Risks and Open Questions

**The model over-helps.** Even with file-write enforcement, the model can dictate too much in chat. Mitigation is prompt policy plus outgoing-message screening, tuned against beta transcripts.

**False negatives in review.** The biggest trust risk is rejecting a valid-but-unexpected solution. Evaluating the goal rather than a reference and verifying when uncertain mitigate it, but review quality must be graded first in beta transcripts.

**Step sizing is hard.** Too small is patronizing; too big is discouraging. The profile helps, but dynamic adjustment based on time per step, error rate, and `/stuck` usage is likely needed early.

**Watcher reliability.** Network drives, unusual editors, and generated files can produce noisy or missed events. `/done` authority and git snapshots bound the damage: worst case, Nina behaves as if watching were off.

**Running arbitrary commands.** User confirmation is the MVP safety model. A container-based runner is a v2 candidate, especially for generated learning projects.

**Latency vs. flow.** Review must feel snappy. The levers are streaming where possible, fast-model triage, and reviewing diffs rather than whole files. Low-dial message screening adds delay and must stay on the fast tier.
