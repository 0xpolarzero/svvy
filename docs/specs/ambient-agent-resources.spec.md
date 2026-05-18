# Ambient Agent Resources Spec

## Status

- Date: 2026-05-19
- Status: adopted direction for opt-in ambient agent resources
- Scope of this document:
  - define provider-neutral ambient agent resource categories
  - define which resource categories are disabled by default
  - define how users opt in through settings
  - define how pi resources map into the provider-neutral model
  - define the standards for future coding-agent hosts that expose similar resource systems

## Purpose

`svvy` owns the actor contract for orchestrator, handler-thread, and workflow task-agent surfaces.

Actor prompts and callable APIs must come from `svvy`'s Context Library, generated declarations, and explicit tool registries. Native coding-agent resource systems are useful, but they must not silently widen a surface's tools, commands, prompt context, UI behavior, provider adapters, or execution policy just because files exist in a workspace or global agent directory.

Ambient agent resources are therefore default-off. The user may explicitly enable them through settings, with clear provenance and category-level scope.

This is provider-neutral. Pi is the first concrete host, but the same product rule applies to Codex, Claude-style integrations, Smithers-backed task agents, and any later coding-agent host that discovers local tools, skills, commands, hooks, prompts, packages, or settings.

## Source Evidence

This spec is grounded in current repo-local references.

- pi's `DefaultResourceLoader` supplies extensions, skills, prompt templates, themes, and context files. See `docs/references/pi-mono/packages/coding-agent/src/core/resource-loader.ts` and `docs/references/pi-mono/packages/coding-agent/docs/sdk.md`.
- pi discovers `AGENTS.md` and `CLAUDE.md` as runtime standards from the app agent directory and cwd ancestors. See `docs/references/pi-mono/packages/coding-agent/src/core/resource-loader.ts`.
- pi discovers `SYSTEM.md` and `APPEND_SYSTEM.md` as native prompt replacement or append files, but `svvy` already overrides those through the real system prompt path. See `docs/references/pi-mono/packages/coding-agent/examples/sdk/03-custom-prompt.ts`, `docs/prd.md`, and `docs/features.ts`.
- pi discovers extensions, skills, prompts, and themes from project `.pi/*`, the app agent directory, package resources, settings paths, and explicit loader paths. See `docs/references/pi-mono/packages/coding-agent/src/core/package-manager.ts` and `docs/references/pi-mono/packages/coding-agent/docs/settings.md`.
- pi extensions can register tools, commands, shortcuts, flags, hooks, UI surfaces, rendering behavior, persistence, and event interception. See `docs/references/pi-mono/packages/coding-agent/docs/extensions.md`.
- pi skills are on-demand capability packages exposed through skill metadata and slash invocation. See `docs/references/pi-mono/packages/coding-agent/docs/skills.md`.
- pi prompt templates are Markdown prompt snippets invoked through slash commands. See `docs/references/pi-mono/packages/coding-agent/docs/prompt-templates.md`.
- pi themes are UI resources, not agent behavior by themselves. See `docs/references/pi-mono/packages/coding-agent/docs/themes.md`.
- pi settings can configure packages, resource paths, providers, models, shell, sessions, UI, retries, compaction, images, and message delivery. See `docs/references/pi-mono/packages/coding-agent/docs/settings.md`, `docs/references/pi-mono/packages/coding-agent/docs/providers.md`, and `docs/references/pi-mono/packages/coding-agent/docs/models.md`.
- Codex local protocol references expose analogous concepts: tools, MCP servers, resources, resource templates, skills, hooks, plugins, commands, config, providers, and auth state. See `docs/references/codex/codex-rs/app-server-protocol/schema/typescript/v2/`.
- Smithers references expose durable workflow resources, task agents, prompts, run options, approvals, retries, outputs, and persisted workflow state. See `docs/references/smithers/`.

The product term "ambient agent resources" is a `svvy` category name. It is not assumed to be a native term used by every host.

## Definitions

### Ambient Agent Resource

A capability, prompt asset, UI asset, package, command, hook, setting, provider adapter, credential source, or execution policy discovered from a coding-agent host's native configuration instead of from explicit `svvy` product settings.

Examples include project `.pi/extensions`, global skills, prompt templates, MCP servers, hooks, slash commands, package manifests, custom provider adapters, and host-specific settings files.

### Runtime Standards

Plain instruction files that define repository or user standards and are appended as visible context.

For pi, the adopted runtime standards are:

- `AGENTS.md`
- `CLAUDE.md`

Runtime standards are not treated as behavior-changing ambient resources. They remain part of the Context Library generated-context preview, with exact source path, order, content, and hash. Surface prompt freshness checks compare the bound standards hashes with the current standards hashes.

### Native Prompt Replacement

Host-specific files or settings that replace or append the system prompt outside `svvy`'s Context Library.

For pi, these are:

- `.pi/SYSTEM.md`
- `SYSTEM.md` in the agent directory
- `.pi/APPEND_SYSTEM.md`
- `APPEND_SYSTEM.md` in the agent directory

`svvy` does not load these into actor prompts. The Context Library and generated prompt parts own system prompt composition.

## Provider-Neutral Categories

`svvy` models ambient resources with these categories.

### Callable Capabilities

Tools or APIs callable by the model.

Examples:

- pi extension tools
- MCP tools
- Smithers runtime tools
- host built-in tools
- generated `execute_typescript` APIs
- svvy direct tools

Standard: callable capabilities are default-off unless registered by `svvy` or explicitly enabled by the user. Any enabled callable capability must appear in the actor-specific generated declaration for the exact actor that may call it.

### Runtime Extensions And Packages

Executable modules or packages that can modify agent behavior.

Examples:

- pi extensions
- pi packages
- Codex plugins
- MCP server packages
- future Claude-style plugins or extension packs

Standard: executable ambient resources are default-off. Discovery for disabled resources must not execute the resource. Enabling requires explicit user action and must record source path or package identity.

### Skills

On-demand instruction or workflow packages with metadata, references, scripts, assets, or dependencies.

Examples:

- pi skills
- Codex skills
- local `.agents/skills` resources

Standard: skills are default-off as agent-invokable capabilities. If enabled, their metadata may be exposed only to selected actor classes, and any full skill loading must follow the host's native skill rules while remaining visible in `svvy` settings and prompt previews.

### Prompt Assets

Prompt templates, snippets, recipes, generated declarations, and native prompt overrides.

Examples:

- pi prompt templates
- slash prompt snippets
- Context Library instruction blocks
- Context Library context packs
- generated tool/schema declarations
- native system prompt replacement files

Standard: `svvy` Context Library records and generated declarations are product-owned, not ambient. Native prompt templates are default-off. Native prompt replacement is ignored unless a future product feature imports it as editable Context Library content through explicit user action.

### Commands And Hooks

User-invoked commands or lifecycle hooks that can route work, mutate input, intercept tools, or alter runtime behavior.

Examples:

- pi slash commands
- extension commands
- prompt-template slash commands
- Codex hooks
- tool-call hooks
- shell or bash spawn hooks
- command palette actions

Standard: product commands are registered by `svvy`. Ambient commands and hooks are default-off. Enabled commands must route through `svvy` command/action registration or a clearly scoped host bridge; enabled hooks must declare the lifecycle point they affect.

### UI And Interaction Resources

Resources that affect presentation or interaction rather than model reasoning directly.

Examples:

- themes
- keybindings
- editor replacements
- status/footer/widgets/overlays
- renderer components

Standard: UI resources are default-off for `svvy` surfaces unless the product explicitly supports that host bridge. Themes are not allowed to change actor prompt or tool policy.

### Provider And Model Adapters

Host-specific provider, model, compatibility, or reasoning settings.

Examples:

- custom provider extensions
- model registry files
- context-window metadata
- modality metadata
- provider fallback rules

Standard: `svvy` provider settings remain app-owned. Ambient provider adapters are default-off and may be enabled only when the product can show the provider identity, auth path, model metadata, and affected actor settings.

### Credentials And Auth State

Secrets, OAuth tokens, API keys, environment-variable resolvers, auth files, and MCP auth state.

Standard: credentials are never imported implicitly from ambient host settings unless the product explicitly owns that auth integration. Settings UI must distinguish using an existing host credential from storing a `svvy` credential.

### Execution Policy

Settings that change what code may run, where it runs, how shell commands execute, network access, sandboxing, approvals, timeouts, retries, and concurrency.

Standard: `svvy` execution policy is product-owned. Ambient execution policy is default-off unless explicitly mapped into a `svvy` setting. Enabling an ambient tool or extension does not automatically enable its host's unrelated execution policy.

### Runtime State Resources

Session ids, cwd, workspace ids, queued messages, compaction state, workflow run ids, approvals, durable artifacts, logs, command facts, and similar state.

Standard: durable state remains owned by the system that produced it. `svvy` stores only product-specific facts it cannot derive from pi or Smithers durable state.

## Default Policy

The default policy is:

- preserve runtime standards as visible plain context
- ignore native prompt replacement and append files
- disable ambient callable capabilities
- disable ambient executable extensions and packages
- disable ambient skills as model-invokable resources
- disable ambient prompt templates and slash prompt snippets
- disable ambient commands and hooks
- disable ambient UI resources for `svvy` surfaces
- disable ambient provider/model adapters and credentials
- ignore ambient execution policy unless mapped to a `svvy` setting

The default state must be deterministic for a given `svvy` configuration. Adding `.pi/extensions`, `.pi/skills`, `.pi/prompts`, `.pi/themes`, `.agents/skills`, host plugin directories, hooks, or package manifests to a workspace must not change an actor's prompt, tools, commands, UI, provider, auth, or execution behavior until the user opts in.

## Opt-In Settings Model

`svvy` needs an Ambient Agent Resources settings surface.

The settings model is scoped by:

- workspace
- provider or host, such as pi, Codex, Claude-style host, or Smithers task-agent host
- actor class, such as orchestrator, handler thread, or workflow task agent
- resource category
- source path or package identity

Settings must support:

- category-level disabled-by-default toggles
- source-level enablement for discovered paths or packages
- actor-level allowlists for callable resources
- provenance display for every enabled resource
- warnings for executable resources and credential-affecting resources
- prompt freshness invalidation when enabled prompt-affecting resources change
- command/action registry refresh when enabled command resources change
- active session behavior that applies only at safe turn boundaries or through queued control work

Disabled resources may be listed only when they can be discovered without executing untrusted code. If safe metadata discovery is not available for a host category, the settings surface should allow path/package opt-in without previewing executable metadata first.

## Pi Mapping

Pi resources map into the provider-neutral categories as follows.

| Pi resource | Category | Default |
| --- | --- | --- |
| `AGENTS.md`, `CLAUDE.md` | Runtime standards | Preserved and visible |
| `.pi/SYSTEM.md`, global `SYSTEM.md` | Native prompt replacement | Ignored |
| `.pi/APPEND_SYSTEM.md`, global `APPEND_SYSTEM.md` | Native prompt replacement | Ignored |
| `.pi/extensions`, global extensions, package extensions, settings extension paths | Runtime extensions; callable capabilities; commands; hooks; UI; provider adapters | Disabled |
| `.pi/skills`, `.agents/skills`, global skills, package skills, settings skill paths | Skills; commands when slash-invoked | Disabled |
| `.pi/prompts`, global prompts, package prompts, settings prompt paths | Prompt assets; commands when slash-invoked | Disabled |
| `.pi/themes`, global themes, package themes, settings theme paths | UI and interaction resources | Disabled |
| pi package manifests | Runtime extensions and packages | Disabled |
| pi model/provider settings and custom provider extensions | Provider and model adapters; credentials | Disabled unless represented by `svvy` provider settings |
| pi shell, retry, compaction, message-delivery, and UI settings | Execution policy; UI and interaction resources | Disabled unless mapped to `svvy` settings |

Pi implementation should use `DefaultResourceLoader` controls to enforce the policy:

- `systemPromptOverride` supplies the `svvy` actor prompt.
- `appendSystemPromptOverride` returns an empty list.
- `agentsFilesOverride` is not used for the default runtime-standards path because `svvy` preserves `AGENTS.md` and `CLAUDE.md`.
- `noExtensions`, `noSkills`, `noPromptTemplates`, and `noThemes` are true unless settings opt into the corresponding category for that host and actor.
- Explicit additional paths are used only for settings-enabled sources.
- Override hooks may be used to filter loaded resources to the enabled set.

## Actor Surface Rules

Opting into a resource never bypasses actor boundaries.

- Orchestrator resources must not automatically reach handler threads or workflow task agents.
- Handler resources must not automatically reach orchestrators or workflow task agents.
- Workflow task-agent resources must remain task-local and Smithers-owned where applicable.
- Callable resources must be represented in the generated API block for the actor that can call them.
- Hidden callable tools are not allowed.
- Prompt-affecting resources must appear in prompt previews and prompt drift checks.
- Command-affecting resources must appear in the command/action registry or a host-specific command bridge.

## Implementation Plan

1. Add ambient-resource settings schema and UI with all categories default-off.
2. Add host adapters that map native resources into the provider-neutral categories.
3. For pi, make orchestrator, handler-thread, and workflow task-agent resource loaders default to no extensions, no skills, no prompt templates, and no themes while preserving runtime standards and prompt overrides.
4. Add source-level opt-in for pi resource categories, using explicit paths and loader override filters.
5. Update actor prompt generation so enabled prompt-affecting resources appear in generated previews and prompt freshness hashes.
6. Update generated callable API blocks so enabled callable resources are visible only to allowed actors.
7. Update command/action discovery so enabled ambient commands appear through product command routing.
8. Add tests that prove disabled ambient pi resources do not change prompt, tool, command, or UI behavior, and enabled resources appear only in the selected actor/category scope.

## Non-Goals

- Do not silently import native host prompt replacement files into the Context Library.
- Do not execute disabled extensions or packages for discovery.
- Do not create compatibility aliases for old ambient behavior.
- Do not let ambient resources mutate actor surfaces without generated declarations or visible settings provenance.
- Do not use ambient provider credentials without explicit product-level auth integration.
