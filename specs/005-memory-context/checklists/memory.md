# Memory Checklist: Memory Context Indexing

**Purpose**: Validate the quality, clarity, completeness, and consistency of the end-to-end memory requirements before planning and implementation
**Created**: 2026-06-09
**Feature**: [Link to spec.md](../spec.md)

**Note**: This checklist evaluates the written requirements, not implementation behavior.

## Requirement Completeness

- [ ] CHK001 Are the configurable timeline-window settings fully specified for all three layers: raw notes, weekly summaries, and monthly summaries? [Completeness, Spec §FR-005, §FR-006, §FR-007]
- [ ] CHK002 Does the spec define what happens when the configured monthly-summary window is not set to “all remaining months”? [Completeness, Spec §FR-007, Spec §FR-010]
- [ ] CHK003 Are the boundaries between raw-note, weekly-summary, and monthly-summary windows explicitly defined so no time period is left unspecified or double-covered? [Completeness, Spec §FR-008, §FR-009, §FR-010]
- [ ] CHK004 Are requirements defined for how missing weekly or monthly summaries affect default context assembly? [Completeness, Spec §FR-054]
- [ ] CHK005 Are the requirements for automatic weekly and monthly summary creation complete for both active use and catch-up after inactivity? [Completeness, Spec §FR-019, §FR-020, Edge Cases]
- [ ] CHK006 Are the LLM search-tool requirements complete for both keyword search and time-range search outputs? [Completeness, Spec §FR-044, §FR-045, §FR-046, §FR-047]
- [ ] CHK007 Does the spec define whether time-range search returns only raw notes, only summaries, or both when both exist in-range? [Completeness, Spec §FR-053]
- [ ] CHK008 Are requirements defined for user-adjustable settings behavior when one or more timeline layers are configured to zero months? [Completeness, Edge Cases]

## Requirement Clarity

- [ ] CHK009 Is the term “month” clarified well enough to determine whether settings use rolling periods, completed calendar months, or another rule for each timeline layer? [Clarity, Spec §FR-005 through §FR-010, Assumptions]
- [ ] CHK010 Is “all remaining months” defined precisely enough to avoid different interpretations for very old history retention in default context? [Clarity, Spec §FR-007, Assumptions]
- [ ] CHK011 Is “keyword-based memory search” specific enough to distinguish requirement intent from generic semantic retrieval or free-text filtering? [Clarity, Spec §FR-044, Clarifications §2026-06-09]
- [ ] CHK012 Is “best available memory” defined clearly enough to determine which source should be preferred when summaries are missing or stale? [Clarity, Spec §FR-054, Edge Cases]
- [ ] CHK013 Are “outdated” and “stale” differentiated clearly for summaries versus cache entries so the terms cannot be confused during implementation or review? [Clarity, Spec §FR-031, §FR-052]
- [ ] CHK014 Is the phrase “relevant notes” in search-tool requirements supported by objective ranking or filtering expectations rather than subjective interpretation alone? [Clarity, Spec §FR-045]

## Requirement Consistency

- [ ] CHK015 Do the clarified defaults in the Clarifications section align exactly with the assumptions and functional requirements for timeline windows? [Consistency, Clarifications §2026-06-09, Spec §FR-005 through §FR-010, Assumptions]
- [ ] CHK016 Are the cache identity and invalidation requirements consistent with the newly configurable timeline-window settings? [Consistency, Spec §FR-029, §FR-037]
- [ ] CHK017 Do the search-tool requirements remain consistent with the default assembled-context model, rather than conflicting with it as an alternative source-of-truth? [Consistency, Spec §FR-044 through §FR-047, User Story 3]
- [ ] CHK018 Are the requirements for removing conversation summaries consistent across user stories, edge cases, and functional requirements? [Consistency, Spec §FR-042, §FR-043, Edge Cases, User Story 1]
- [ ] CHK019 Do the success criteria for configured timeline windows align with the functional requirements without reverting to fixed one-month/two-month assumptions? [Consistency, Spec §SC-001, Spec §FR-005 through §FR-010]

## Acceptance Criteria Quality

- [ ] CHK020 Can the configured timeline-window structure be objectively verified from the current success criteria without relying on implementation knowledge? [Measurability, Spec §SC-001]
- [ ] CHK021 Are weekly and monthly rollup timeliness expectations quantified clearly enough to determine pass/fail for delayed generation? [Measurability, Spec §SC-002, §SC-003]
- [ ] CHK022 Are cache invalidation expectations for timeline-setting changes measurable rather than purely descriptive? [Measurability, Spec §SC-010, Spec §FR-037]
- [ ] CHK023 Are search-tool outcome expectations measurable enough to distinguish acceptable retrieval quality from vague “relevance”? [Measurability, Spec §SC-006, §SC-007]

## Scenario Coverage

- [ ] CHK024 Are primary requirements defined for all three major flows: default context assembly, automatic rollup generation, and on-demand memory search? [Coverage, User Stories 1-3]
- [ ] CHK025 Are alternate-flow requirements defined for profiles with only recent notes, only old notes, or mixed history spanning all timeline layers? [Coverage, Gap]
- [ ] CHK026 Are exception-flow requirements defined for failures in summary generation, cache refresh, and search with no results? [Coverage, Edge Cases, Spec §FR-031 through §FR-037, §FR-054]
- [ ] CHK027 Are recovery requirements defined for how the system returns to a trusted summary state after underlying notes change? [Coverage, Recovery Flow, Spec §FR-052]
- [ ] CHK028 Are requirements specified for profiles whose history extends beyond the configured monthly-summary window, if such a limit is set? [Coverage, Spec §FR-007, Edge Cases]

## Edge Case Coverage

- [ ] CHK029 Are boundary conditions defined for transitions exactly at week and month cutovers so summary ownership is unambiguous? [Edge Case, Gap]
- [ ] CHK030 Are requirements defined for duplicate-prevention rules across both weekly and monthly summary generation? [Edge Case, Spec §FR-050, §FR-051]
- [ ] CHK031 Are requirements defined for conflicting conditions where a summary is missing, stale, and also within an active search result range? [Edge Case, Gap]
- [ ] CHK032 Does the spec define fallback expectations when vector-backed keyword search is temporarily unavailable? [Edge Case, Edge Cases, Spec §FR-014, §FR-044]

## Non-Functional Requirements

- [ ] CHK033 Are performance requirements defined specifically enough for the expanded timeline-based context model, rather than only in broad qualitative terms? [Non-Functional, Spec §NFR-001, Gap]
- [ ] CHK034 Are reliability requirements defined for automatic rollup generation under missed sessions, delayed processing, and partial failures? [Non-Functional, Spec §NFR-002, Spec §SC-002, §SC-003]
- [ ] CHK035 Are observability requirements specific enough to show what rollup and retrieval signals maintainers need for diagnosis? [Non-Functional, Spec §NFR-004, User Story 5]
- [ ] CHK036 Are UX consistency requirements defined for how settings changes affect later context behavior, especially across sessions and restarts? [Non-Functional, Spec §NFR-003, Spec §FR-011]

## Dependencies & Assumptions

- [ ] CHK037 Are assumptions about rolling periods versus completed calendar periods validated against the functional requirements, rather than left as implicit design choices? [Assumption, Assumptions]
- [ ] CHK038 Are dependencies on existing note extraction, vector indexing, and persistent storage documented without leaving critical behavior unspecified? [Dependency, Spec §FR-002, §FR-003, §FR-011]
- [ ] CHK039 Does the spec clearly distinguish user-facing settings requirements from assistant-facing search-tool requirements so actor boundaries are not assumed? [Assumption, User Story 1, User Story 3, Spec §FR-044, §FR-046]

## Ambiguities & Conflicts

- [ ] CHK040 Is any remaining use of “last month” or “older months” normalized to the configurable timeline-window terminology to avoid contradictory wording? [Ambiguity, Conflict, User Story 1, Assumptions]
- [ ] CHK041 Do plan and tasks documents still reference the earlier cache-only scope in ways that conflict with the broadened spec scope? [Conflict, Plan Summary, Tasks Scope]
- [ ] CHK042 Is a traceable requirement reference present for each major risk area: timeline settings, rollup creation, search tools, cache behavior, and conversation-summary removal? [Traceability, Spec §FR-005 through §FR-054]

## Notes

- Author-focused checklist for PR-quality requirements review at standard depth.
- Focus areas selected: end-to-end memory spec, including context composition, rollups, search tools, and cache interactions.
- Intended user: spec author.
- Review this checklist against `spec.md` first; use `plan.md` and `tasks.md` only to spot scope drift or missing traceability.
