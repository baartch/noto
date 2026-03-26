<!--
  Sync Impact Report
  ==================
  Version change: 1.0.0 → 1.1.0
  Modified principles:
    - IV. Persistent Contextual Memory (storage guidance expanded from
      markdown-only to SQLite-first, LLM-retrieval-optimized records)
    - V. Simplicity and Minimal Dependencies (clarified: minimal deps with
      pragmatic built-in/local persistence)
  Added sections: None
  Removed sections: None
  Templates requiring updates:
    - .specify/templates/plan-template.md ✅ reviewed (no changes needed;
      Constitution Check remains generic)
    - .specify/templates/spec-template.md ✅ reviewed (no changes needed)
    - .specify/templates/tasks-template.md ✅ reviewed (no changes needed)
    - .specify/templates/checklist-template.md ✅ reviewed (no changes needed)
    - .specify/templates/agent-file-template.md ✅ reviewed (no changes needed)
    - No commands/ directory exists; nothing to check.
  Follow-up TODOs: None
-->

# Artman Constitution

## Core Principles

### I. Local-First Privacy

All user data — conversation history, notes, configuration —
MUST remain on the user's local machine. No telemetry, no cloud
sync, no external storage of personal content. The user owns
their data unconditionally.

Rationale: The chatbot accumulates deeply personal context
(goals, struggles, progress). Trust requires absolute local
data sovereignty.

### II. LLM-Agnostic Provider Layer

The system MUST support multiple LLM backends (GitHub Models,
OpenRouter, OpenAI-compatible APIs, local models) through a
unified provider interface. Switching providers MUST NOT require
code changes beyond configuration (env vars or config file).

Rationale: Provider lock-in limits accessibility and increases
cost risk. Users MUST choose the model that fits their budget,
privacy stance, and quality needs.

### III. System-Prompt-Driven Personality

All domain-specific behavior (consulting persona, tone,
expertise area) MUST be defined exclusively in an external
Markdown system-prompt file. The core application MUST be
domain-agnostic; no hardcoded references to music, art
management, or any specific consulting niche.

Rationale: A single system-prompt swap MUST transform the
chatbot from an Artist Manager to a fitness coach, career
advisor, or any other consulting role — zero code changes.

### IV. Persistent Contextual Memory

The chatbot MUST maintain structured local notes that capture
key facts, decisions, progress, and open topics from every
conversation. Primary storage MUST be a local single-file
SQLite database optimized for retrieval (schema + indexes,
including FTS where useful). Markdown exports/imports MAY be
supported for portability, but SQLite is the source of truth.
The LLM MUST receive relevant prior notes as context in each
conversation to provide continuity.

Rationale: Long-term value comes from the chatbot knowing the
user better over time. SQLite keeps memory local, queryable,
and scalable without introducing server complexity.

### V. Simplicity and Minimal Dependencies

Start with the simplest viable implementation. Prefer standard
library and lightweight local components. Every dependency MUST
be justified by a clear need that cannot be met with
reasonable effort in-house. The CLI interface MUST be
straightforward: launch, chat, quit.

Rationale: This is a personal tool. Complexity is the enemy
of reliability and long-term maintainability for a solo or
small-team project.

## Architecture Constraints

- **CLI-only interface**: No web UI, no desktop GUI in v1.
  Text in/out via terminal.
- **Single-user**: No authentication, no multi-tenancy.
  One local installation = one user.
- **Configuration via files**: Provider credentials in env vars
  or a local config file. System prompt in a Markdown file.
  Local data path configurable.
- **Notes storage**: Single-file SQLite database
  (`~/.artman/memory.db` default or configurable). Schema MUST
  support timestamps, tags/topics, session linkage, and note
  content. Full-text indexes SHOULD be used for fast context
  retrieval.
- **Conversation flow**: On each user message the system MUST
  (1) load the system prompt, (2) retrieve relevant notes from
  SQLite, (3) send context + user message to the LLM,
  (4) present the response, (5) update notes with new insights,
  action items, and progress markers.
- **Vector storage policy**: Local vector index is OPTIONAL and
  MUST remain secondary to SQLite source-of-truth records. Do
  not require vector DB in v1.

## Development Workflow

- **Language**: Go (Golang).
- **Testing**: Unit tests for provider abstraction, storage
  access, note extraction, and prompt assembly. Integration
  tests for end-to-end conversation flow with a mock LLM.
- **Code organization**: Keep files under ~500 LOC. Split
  into clear modules: `cli`, `provider`, `memory`, `prompt`.
- **Commits**: Conventional commits. No commit without
  explicit user instruction.

## Governance

This constitution is the highest-authority document for the
Artman project. All implementation decisions, pull requests,
and design changes MUST be consistent with the principles
above.

Amendment procedure:
1. Propose change with rationale.
2. Document impact on existing code and storage format.
3. Update constitution version per semver rules:
   - MAJOR: Principle removal or incompatible redefinition.
   - MINOR: New principle or material expansion.
   - PATCH: Clarification or wording fix.
4. Update `LAST_AMENDED_DATE`.

Compliance: Every plan and spec MUST include a Constitution
Check section verifying alignment with these principles.

**Version**: 1.1.0 | **Ratified**: 2026-03-26 | **Last Amended**: 2026-03-26
