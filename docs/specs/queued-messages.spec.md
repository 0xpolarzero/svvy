# Queued Messages Spec

## Status

- Date: 2026-05-15
- Status: adopted product direction for queued follow-up user messages
- Scope: composer sends that target an already-running orchestrator or handler-thread surface

## Purpose

`svvy` needs a clear way for a user to add a follow-up while a surface is still running.

The product goal is not concurrent turns, hidden steering, or a second terminal-like input loop. The goal is a visible, ordered, surface-local queue:

- the running turn keeps ownership until it settles or is cancelled
- the user's submitted follow-up is accepted instead of lost
- the queued follow-up remains visible and editable before delivery
- the queued follow-up starts the next real turn on that same surface

## Reading Rules

This document separates evidence and product design into three labels:

- `Public Codex fact`: directly supported by public OpenAI Codex material
- `pi implementation fact`: supported by the local pi reference under `docs/references/pi-mono`
- `PRD inference`: adopted `svvy` behavior inferred from the PRD and product architecture

Public Codex facts are product-context inputs only. They are not evidence of pi internals or a requirement to clone Codex UI behavior exactly.

pi implementation facts describe available substrate behavior. They are not automatically exposed as `svvy` product behavior unless this spec adopts them.

## Source Context

### Public Codex facts

- `Public Codex fact`: OpenAI describes Codex as a coding agent that helps users write, review, and ship code, and the Codex app as a command center for agentic coding with parallel work across projects. Source: [Codex product page](https://openai.com/codex).
- `Public Codex fact`: OpenAI describes Codex as spanning Codex CLI, Codex Cloud, and the Codex VS Code extension, with an agent loop that prepares input from the user's prompt, calls a model, and uses tools. Source: [Unrolling the Codex agent loop](https://openai.com/index/unrolling-the-codex-agent-loop/).
- `Public Codex fact`: Codex cloud can work on many tasks in the background, in parallel, in its own cloud environment. Source: [Codex cloud docs](https://platform.openai.com/docs/codex).

These facts support a product expectation that users can keep giving instructions while agentic work is underway. They do not specify `svvy`'s exact queued-message semantics.

### pi implementation facts

- `pi implementation fact`: pi has keybinding names for queueing a follow-up message and restoring queued messages: `app.message.followUp` and `app.message.dequeue`.
- `pi implementation fact`: pi extension APIs expose `sendUserMessage(content, { deliverAs })` with `deliverAs: "steer"` and `deliverAs: "followUp"` when the agent is streaming.
- `pi implementation fact`: pi custom messages also support `deliverAs: "nextTurn"`.
- `pi implementation fact`: pi's agent core distinguishes steering messages from follow-up messages; follow-up messages are checked when the agent has no more tool calls and no steering messages.
- `pi implementation fact`: pi can clear queued steering and follow-up messages and report whether queued messages exist.

These facts make queue-backed behavior feasible through pi's runtime seam. `svvy` adopts only the follow-up queue as a first-class user-facing behavior.

Local pi references:

- `docs/references/pi-mono/packages/coding-agent/src/core/keybindings.ts`
- `docs/references/pi-mono/packages/coding-agent/docs/extensions.md`
- `docs/references/pi-mono/packages/coding-agent/src/core/agent-session.ts`
- `docs/references/pi-mono/packages/agent/README.md`

### PRD inferences

- `PRD inference`: each interactive surface has one prompt lock, so a running turn must not be joined by another concurrent user turn on the same surface.
- `PRD inference`: each surface is addressed by `surfacePiSessionId`; queued messages are therefore surface-local, not global to the workspace and not panel-local.
- `PRD inference`: Dockview panels are views over surfaces. A queued message belongs to the target surface even if zero, one, or many panels show it.
- `PRD inference`: committed conversation history remains in pi's session history. A queued message is not committed transcript history until it is delivered as the next real user message.
- `PRD inference`: prompt history records explicit user submissions. A queued message is an explicit submission and should be eligible for prompt history at queue time, even if delivery happens later.

## Adopted Product Behavior

`svvy` treats ordinary composer submits as queued follow-up user messages. The queue row also exposes an explicit `Steer` action for the uncommon case where the user wants that queued text delivered through pi/Codex-style steering at the next safe boundary of the active turn.

When a user submits from a composer:

- if the target surface is idle, the message starts a normal turn immediately
- if the target surface already has an active turn, the message is queued for that same surface
- the active turn continues until it naturally settles or is cancelled
- the queued message starts the next real turn only after the active turn settles and the surface prompt lock is released

Ordinary queued composer sends must not:

- inject instructions before the current assistant turn finishes its already accepted tool work
- skip remaining tool calls in the current assistant message
- open a concurrent turn on the same surface
- retarget itself to the orchestrator just because the focused panel changed
- become an inline transcript message before delivery

The `Steer` row action is separate from ordinary queued delivery. It commits the selected row into a locked `steering` state and submits that text through pi's steering queue when the surface is active. The row remains visible in the queue projection without edit, delete, reorder, or steer controls until pi either accepts it as a real steering user message or rejects it. If the surface is idle, the closest equivalent behavior is to run that selected queued message next as a normal turn. If steering is rejected, `svvy` restores the row to the front of the durable queue.

## Queue Ownership

A queued message record belongs to exactly one interactive surface.

Required identity:

- `workspaceSessionId`
- `surfacePiSessionId`
- `threadId` when the target surface is a handler thread
- `queuedMessageId`

The queue is ordered per `surfacePiSessionId`. Queue ordering is FIFO unless the user explicitly edits, removes, or reorders messages through future queue-management UI.

Queued messages are not Dockview panel state. If two panels show the same surface, both render the same queue projection. If no panel shows the surface, the queue still belongs to the live surface and durable workspace state.

## Queue Lifecycle

Queued message status:

```ts
type QueuedMessageStatus =
  | "queued"
  | "steering"
  | "dispatching"
  | "delivered"
  | "cancelled";
```

Lifecycle rules:

- `queued`: accepted while a surface was active and waiting for the current turn to settle
- `steering`: selected for pi steering, locked in the UI, and waiting for pi accept/reject
- `dispatching`: selected as the next message for the surface and being submitted through pi
- `delivered`: committed as the next real user message in pi's session history
- `cancelled`: removed by the user before delivery or dropped because the owning surface was explicitly closed in a way that discards queued work

The durable record should keep:

- submitted text exactly as sent
- composer attachments or mention-link serialized text according to their own specs
- creation, update, delivery, and cancellation timestamps
- source panel id for diagnostics only, not for ownership
- prompt-history entry linkage when available

## Delivery Semantics

When the current turn settles, the owning surface wakes the queue runner for its `surfacePiSessionId`.
Queue delivery is owned by that surface runtime, not by any Dockview pane, workspace tab, or visual
instance. Waking the runner is idempotent; if a runner is already scheduled or active for the
surface, additional wakes do not start another delivery loop.

If the queue has at least one queued message:

1. atomically claim the first `queued` message and mark it `dispatching`
2. submit it as the next real user message to that same pi surface
3. create a normal turn record for the delivery
4. mark it `delivered` once pi commits the queued user message into the surface history

If delivery fails before pi accepts the message, the message returns to the front of the durable `queued` list.

If delivery starts and the resulting turn later fails, the queued message remains `delivered`; the turn failure belongs to the normal turn lifecycle.

## Cancellation And Restore

Cancelling the active turn and restoring queued text are separate actions.

The product should support:

- cancelling the active turn without silently deleting queued messages
- removing an individual queued message before delivery
- restoring one or all queued messages into the composer for editing before delivery

The pi TUI's `Alt+Up` restore behavior is a useful reference, but `svvy`'s desktop shortcut and UI affordance should come from the product shortcut registry and composer design rather than copying TUI keybindings directly.

## Prompt History

Queued messages are user-authored explicit submissions.

That means:

- a non-empty queued message is written to workspace prompt history at queue time
- delivery later must not create a duplicate prompt-history entry
- restoring and editing a queued message before delivery creates a new history entry only when the edited text is submitted again

## Projection

Surface UI should show queued messages near the composer for the target surface.

Projection should make clear:

- how many messages are queued
- which surface owns them
- their delivery order
- whether the current surface is running, waiting, or ready
- whether a message is queued for normal follow-up or has been selected for steering

Queued rows render as a compact vertical list directly above attachment chips and the textarea. Rows use single-line ellipsized message text, centered controls, and dense workbench row sizing. Editable `queued` rows expose drag reorder, `Steer`, edit, and delete. Drag-hover reorder previews are local renderer state; the durable queue order changes only when the user drops a row into a final changed position. Locked `steering` or `dispatching` rows remain in place but replace the controls with a status indicator and cannot be edited, deleted, steered again, or reordered.

Sidebar rows may show a compact queued-count badge for an open surface, but queued messages do not change the row's lifecycle status to running or waiting by themselves.

## Restart Recovery

Queued messages should survive app restart until they are delivered, cancelled, or restored for editing.

On restore:

- queued messages remain ordered under their owning `surfacePiSessionId`
- if the owning surface is idle and has queued messages, the app may resume delivery after reconstructing the surface runtime and prompt lock state
- if the owning surface is active because a turn or workflow-attention wake-up resumed, queued delivery waits for that active work to settle

Recovery must not infer queued messages from transcript text. The queue is structured product state.

## Explicitly Out Of Scope

- treating ordinary composer submit as pi steering
- global workspace message queues
- queueing messages across multiple target surfaces from one submit
- running two user turns concurrently on one surface
- queuing slash commands or product command-palette actions as transcript text
- treating queued messages as Smithers workflow signals, approvals, or handler handoffs
- preserving queued-message state in Dockview panel layout JSON

## Test Planning

Tests should cover:

- idle send starts immediately
- active-surface send creates a queued message and leaves the active turn alone
- queued messages deliver FIFO after the prompt lock releases
- queued messages stay surface-local across orchestrator and handler-thread surfaces
- queued messages render consistently in duplicated panels for the same surface
- prompt history records queued submissions once
- cancellation does not silently delete queued messages
- queued messages restore across restart without transcript inference
- delivery that fails before pi accepts the message restores the durable queued row
