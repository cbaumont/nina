# Design Document: Nina — an AI Pair Programming Companion

**Status:** Draft v0.1
**Author:** Carlos
**Date:** July 2026

## 1. Summary

Nina is a CLI-based AI pair programming companion built for people who want to *learn* to code, not just have code written for them. It inverts the dominant AI coding assistant model: following classic pair programming practice, the human is the **driver** (the one typing) and the AI is the **navigator** (the one directing, explaining, and reviewing). The AI can scaffold projects, generate boilerplate, and run or debug code, but the substantive typing is done by the user, guided step by step with explanations calibrated to their experience level.

The core hypothesis is that active production of code with expert guidance produces far better learning outcomes than passive consumption of AI-generated code, and that current tools (Copilot, Cursor, Claude Code) are optimized for productivity rather than pedagogy, leaving this niche underserved.

## 2. Goals and Non-Goals

### Goals

The MVP should let a user start a guided coding session in a terminal alongside their preferred editor, work through a task in small instructed steps where they do the typing, and receive explanations of *what* to do, *how* to do it, and *why* it is done that way. It should support both AI-generated learning projects and the user's own existing projects (including "implement a new feature in this codebase" sessions). It should let the user configure their general programming experience, their familiarity with the specific stack, and how much the AI is allowed to type on their behalf. Finally, it should be able to run and debug code, and review what the user typed against what was intended, treating mistakes as teaching moments.

### Non-Goals (for MVP)

Nina is not trying to be a productivity tool; if the user wants the task done fast, other tools exist and we should not compete with them. The MVP will not include an editor extension, a GUI, multi-user/classroom features, structured curricula with progress tracking across sessions, or gamification. It will not attempt to sandbox arbitrary code execution beyond reasonable defaults — the user runs code on their own machine, as they would when learning without an AI.

## 3. Target Users

The primary audience is anyone learning a specific language, framework, or technology. This spans a wide range, which is why experience configuration is a first-class feature rather than an afterthought. Three representative personas:

**The newcomer.** Has never programmed. Needs explanations of fundamentals (what a variable is, what a terminal is), very small steps, and heavy encouragement. Likely working through an AI-generated learning project.

**The switcher.** An experienced developer learning a new stack — say, a Python data scientist learning TypeScript and React. Needs terse instructions, idiom-level explanations ("in TS we'd use a discriminated union here, which plays the role that your Python `Enum` + `match` pattern did"), and analogies to what they already know. Likely working in a real project.

**The upskiller.** Knows the stack basics but wants to level up on a specific area — testing, async, performance, a new framework version. Wants the AI to push best practices and explain trade-offs, not syntax.

## 4. User Experience

### 4.1 The Core Loop

A session follows a repeating rhythm modeled on driver/navigator pair programming:

1. **Orient.** The navigator explains the current goal in context: "Next we need a function that validates the form input before we touch the database. Here's why validation lives here and not in the route handler…"
2. **Instruct.** The navigator gives a concrete, right-sized instruction: which file to open, what to write, at a level of specificity matching the user's dial settings. For a newcomer this might be near-dictation with line-by-line explanation; for a switcher it might be "add a Zod schema for the signup payload — you know JSON Schema, this is the same idea with type inference."
3. **Drive.** The user types in their own editor. Nina watches the file system and detects when the relevant files change (with an explicit "done" command as fallback).
4. **Review.** The navigator reads the diff of what the user actually wrote, compares it against intent, and responds. Correct code gets a brief confirmation and, when valuable, a note on style or an alternative approach. Incorrect code gets Socratic handling by default: point at the symptom, ask a guiding question, escalate to the fix only if the user is stuck (escalation speed is configurable — see 4.3).
5. **Verify.** Where possible, the navigator runs the code or the tests to close the loop with real feedback, and teaches the user to read the output ("this stack trace says the error is on line 14 — what do you notice about the type of `user.id` there?").
6. **Advance.** Move to the next step, periodically zooming out to recap how the pieces fit together.

The session transcript itself is a learning artifact: at the end of a session, Nina can generate a summary of what was built, the concepts covered, and suggested topics to revisit.

### 4.2 Session Types

**Learning project mode.** The user describes what they want to learn ("Rust basics", "REST APIs with FastAPI") and Nina proposes 2–3 project ideas sized to a few sessions, then scaffolds the repository and begins the loop. Generated projects should be *real-feeling* — a small but genuine application rather than exercises — because motivation is a load-bearing part of learning.

**Own-project mode.** The user points Nina at an existing repository and states a goal ("add dark mode", "write tests for the payments module", "I inherited this codebase and want to understand it by extending it"). Nina first performs a codebase exploration phase — reading structure, conventions, and dependencies — and narrates its findings as a guided tour before planning the feature work. This mode doubles as codebase onboarding, which may prove to be a killer use case in its own right (new hires learning a company codebase).

Both modes converge on the same core loop; they differ only in how the task backlog originates.

### 4.3 Configuration: the Profile and the Dial

At first run (and adjustable any time with `/profile`), the user sets:

**General programming experience** — none / beginner / intermediate / professional. Controls how much foundational explanation accompanies instructions.

**Stack familiarity** — per language/framework in play, on the same scale. Controls idiom-level vs. syntax-level explanation, and whether analogies to known stacks are used. Nina should also *infer and update* this over time: if the user breezes through pointer syntax, stop explaining it.

**The typing dial** — how much the AI is permitted to write, as an explicit, user-controlled setting:

| Level | AI may write | Intended for |
|---|---|---|
| 0 — Full manual | Nothing. Instructions only. | Purists; exam prep |
| 1 — Scaffold | Project setup, config files, package installs | Default for learning projects |
| 2 — Boilerplate | Level 1 + repetitive/no-learning-value code (imports, fixtures, type stubs) | Default overall |
| 3 — Collaborative | Level 2 + AI may take the keyboard for stretches the user delegates ("you write the CSS, I'll focus on the logic") | Time-boxed sessions; upskillers |

Two design principles govern the dial. First, *the dial is a ceiling, not a target*: even at level 3, the navigator defaults to the user typing anything with learning value for them specifically. Second, *transparency at the moment of use*: whenever the AI writes code, it says so and explains what it wrote, and anything the AI wrote is fair game for the AI to quiz the user about later.

A related setting controls **hint escalation speed** in the review step — how quickly the navigator moves from "something's off in that function" to "line 12, you're mutating the list while iterating" to showing the fix. Impatient learners and reflective learners need different defaults.

### 4.4 Interaction Design in a Terminal

The CLI must feel like a companion, not a REPL. Key affordances: a persistent status line (current task, step, dial level); markdown rendering with syntax highlighting for instructions; slash commands (`/why` for a deeper explanation of the last instruction, `/stuck` to escalate a hint, `/skip`, `/recap`, `/profile`, `/dial`, `/run`, `/done`); and streaming responses so guidance appears immediately. The user's editor is untouched and unmonitored except for reading file contents — Nina never modifies files outside its permitted dial level, and never without announcing it.

## 5. Architecture

### 5.1 Overview

```
┌─────────────────────────────┐
│  CLI (TUI)                  │  Node.js or Python; Ink/Textual for TUI
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
┌───┴────┐ ┌───┴─────┐ ┌──┴──────────┐
│ LLM    │ │ Workspace│ │ Runner      │
│ client │ │ watcher  │ │ - run code  │
│(Claude │ │ - fs     │ │ - tests     │
│  API)  │ │   events │ │ - capture   │
│        │ │ - diffs  │ │   output    │
└────────┘ └──────────┘ └─────────────┘
```

### 5.2 Components

**Session Engine.** The heart of the system. Maintains a *task plan* (an ordered, editable list of steps generated by the LLM at session start and revised as reality intervenes), the *step state machine* (orient → instruct → drive → review → verify → advance), and the user profile. Critically, the engine — not the model's goodwill — enforces the typing dial: file-write tool calls from the LLM are filtered against the current dial policy, so a level-0 session physically cannot result in AI-written code. Prompt assembly injects the profile, dial policy, task plan, recent diffs, and pedagogical instructions into each model call.

**Workspace Watcher.** Uses native file-system events (chokidar / watchdog) plus git. Nina initializes or uses the repo's git to snapshot state at each step boundary, so "what did the user just write" is a clean diff rather than a guess. Debounced change events trigger the review step; `/done` is the explicit fallback for editors or setups where watching is unreliable.

**Runner.** Executes build/test/run commands with captured stdout/stderr fed back to both the user and the model. Commands are proposed by the model but confirmed by the user before execution (configurable to auto-approve safe classes like the project's own test command). No sandboxing beyond confirmation in MVP; this matches the trust model of running code you're writing on your own machine, but is flagged as a risk in section 8.

**LLM Client.** Claude API via the official SDK, using tool use for structured actions (write_file — dial-filtered, read_file, run_command, update_plan, update_profile). Two model tiers: a strong model for planning, review, and explanation; optionally a fast model for lightweight classification (e.g., "did this change complete the step?"). Session context is managed by summarizing older steps — sessions are long, and the transcript grows fast.

**State & Persistence.** A `.navigator/` directory in the project holds the task plan, profile overrides, session transcripts, and step snapshots, making sessions resumable and giving the end-of-session summary its raw material.

### 5.3 Key Design Decisions

**Git as the source of truth for "what the user did".** Rather than tracking keystrokes or polling file contents, snapshotting at step boundaries and diffing is simple, robust, editor-independent, and additionally teaches good git hygiene by osmosis (Nina can narrate its commits).

**Dial enforcement in the engine, not the prompt.** Prompt-only enforcement will leak — models drift toward being maximally helpful. Making the dial a hard filter on tool calls turns the pedagogy constraint into an architectural guarantee, which is also a differentiator worth stating in marketing terms: *it can't just do it for you*.

**Pedagogy as explicit prompt policy.** The teaching behavior (Socratic review, hint escalation, right-sized steps, "why" alongside "what") is encoded as a structured system prompt assembled from the profile, and is the highest-iteration part of the product. It should live in versioned prompt files, not code, so it can be tuned rapidly against real sessions.

## 6. MVP Scope

The MVP is a single-user CLI, installable via npm or pipx, supporting one or two launch stacks done well (proposal: Python and JavaScript/TypeScript, covering the two largest learner populations) rather than every stack poorly. It includes both session types, the profile, the typing dial with engine-level enforcement, the core loop with file watching and git snapshots, the runner with confirmation, session persistence/resume, and end-of-session summaries. Bring-your-own-API-key for the model backend keeps billing out of scope initially.

Explicitly deferred: additional stacks, long-term learner progress models across projects, spaced-repetition review of past concepts, voice interaction, and any hosted/account infrastructure.

## 7. Success Criteria

For an MVP validating a learning product, the questions are: do users complete sessions rather than abandoning them mid-loop (session completion rate); do they come back (return sessions per user in week one); do they keep the dial low or crank it up and turn the tool into a code generator (dial-level distribution over time — a rising dial is a warning sign that the pedagogy is failing); and, qualitatively, can users explain the code they wrote afterwards (exit-summary self-quiz, opt-in). A small closed beta of 15–25 learners across the three personas, with session transcripts reviewed by hand, will teach more than any metric at this stage.

## 8. Risks and Open Questions

**The model over-helps.** Even with dial enforcement on file writes, the model can effectively write the code by dictating it verbatim in chat at the wrong level of detail. Mitigation is prompt policy plus review-step behavior, and it will need constant tuning; this is the product's central quality problem.

**Step sizing is hard.** Too small is patronizing; too big is discouraging. The profile helps, but dynamic adjustment based on observed struggle (time per step, error rate, `/stuck` usage) is likely needed sooner than expected.

**Watcher reliability.** Network drives, exotic editors, and generated files will produce noisy or missed events. The `/done` fallback and git snapshots bound the damage, but polish here matters for trust.

**Running arbitrary commands.** User confirmation is the MVP safety model; a container-based runner is a candidate for v2, especially for generated learning projects where Nina controls the whole environment anyway.

**Latency vs. flow.** The review step must feel snappy or the loop dies. Streaming, a fast-model triage tier, and reviewing diffs (small) rather than files (large) are the levers.
