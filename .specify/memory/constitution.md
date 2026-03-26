<!--
  Sync Impact Report
  ==================
  Version change: 1.1.0 → 1.2.0
  Modified principles:
    - III. System-Prompt-Driven Personality (expanded: per-profile prompts,
      editable at runtime)
    - IV. Persistent Contextual Memory (expanded: strict profile isolation,
      one SQLite memory per purpose)
    - V. Simplicity and Minimal Dependencies (clarified startup UX for
      profile selection and lifecycle)
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

# Noto Constitution

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
expertise area) MUST be defined in editable external Markdown
system-prompt files, scoped per profile (purpose). The core
application MUST remain domain-agnostic; no hardcoded references
to specific consulting niches.

Rationale: A profile prompt swap MUST transform Noto from an
Artist Manager to any other consulting role without code
changes.

### IV. Persistent Contextual Memory

Noto MUST maintain structured local memory that captures key
facts, decisions, progress, and open topics from every
conversation. Memory storage MUST be one local single-file
SQLite database per profile, optimized for retrieval (schema +
indexes, including FTS where useful). Profile memories MUST be
strictly isolated. The LLM MUST receive relevant prior notes
from the active profile only.

Rationale: Long-term value requires continuity plus isolation.
Each purpose needs dedicated context to avoid cross-domain
contamination.

### V. Simplicity and Minimal Dependencies

Start with the simplest viable implementation. Prefer standard
library and lightweight local components. Every dependency MUST
be justified by a clear need that cannot be met with
reasonable effort in-house. CLI flow MUST remain simple,
including profile selection and chat startup.

Rationale: This is a personal tool. Complexity is the enemy
of reliability and long-term maintainability for a solo or
small-team project.

## Architecture Constraints

- **CLI-only interface**: No web UI, no desktop GUI in v1.
  Text in/out via terminal.
- **Single-user**: No authentication, no multi-tenancy.
  One local installation = one user.
- **Profile model**: A profile represents one purpose/consulting
  context. Each profile MUST have:
  - one SQLite memory DB file
  - one editable `system_prompt.md` file
  - optional per-profile provider/model overrides
- **Storage layout**: Local data root defaults to `~/.noto/`.
  Profiles MUST be stored under a deterministic directory
  structure (e.g., `profiles/<profile-id>/memory.db` and
  `profiles/<profile-id>/system_prompt.md`).
- **Profile lifecycle**: CLI MUST support create/list/select/
  edit-prompt/delete profile operations.
- **Startup behavior**:
  - 0 profiles: prompt user to create one.
  - 1 profile: auto-select it.
  - >1 profiles: require explicit selection or configured
    default.
- **Prompt editing**: System prompt files MUST be user-editable
  at any time via normal file editing workflow (`$EDITOR` or
  direct file path).
- **Deletion policy**: Profiles (including their DB + prompt)
  MUST be deletable locally with explicit confirmation to
  prevent accidental data loss.
- **Conversation flow**: On each user message Noto MUST
  (1) load active profile prompt, (2) retrieve relevant profile
  memory from SQLite, (3) send context + user message to the
  LLM, (4) present response, (5) update active profile memory.
- **Vector storage policy**: Local vector index is OPTIONAL and
  MUST remain secondary to SQLite source-of-truth records. Do
  not require vector DB in v1.

## Development Workflow

- **Language**: Go (Golang).
- **Testing**: Unit tests for provider abstraction, profile
  routing/isolation, storage access, note extraction, and prompt
  assembly. Integration tests for end-to-end conversation flow
  with mock LLM across multiple profiles.
- **Code organization**: Keep files under ~500 LOC. Split
  into clear modules: `cli`, `provider`, `profile`, `memory`,
  `prompt`.
- **Commits**: Conventional commits. No commit without
  explicit user instruction.

## Governance

This constitution is the highest-authority document for the
Noto project. All implementation decisions, pull requests,
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

**Version**: 1.2.0 | **Ratified**: 2026-03-26 | **Last Amended**: 2026-03-26
