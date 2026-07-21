# Specd Workflow Analysis and Improvement Planning

Use **Caveman** to analyze the following files:

* `WORKFLOW-FEEDBACK.md`
* `specd-context-greenfield-debug-analysis.md`
* `unattended-approval-analysis.md`
* `AIDO-WORKFLOW-FEEDBACK.md`

## Objective

Build a complete understanding of the practical issues encountered while using Specd to implement real-world requirements.

The analysis must focus on the friction experienced across the full Specd workflow, including:

* Initial setup and configuration
* Steering the coding agent
* Creating and refining specifications
* Generating and executing tasks
* Completing specifications
* Reopening completed or closed work
* Managing unattended execution and approvals
* Handling debugging and greenfield development
* Using the coding agent for work unrelated to Specd

The current workflow has been difficult to operate at nearly every stage. It requires excessive configuration, repeated steering, and significant manual effort to build and complete specifications.

A major concern is that coding-agent requests unrelated to Specd appear to be interpreted and processed through the Specd workflow. This introduces unnecessary overhead and makes normal repository work harder. Specd must distinguish between:

1. Requests that should follow the Specd lifecycle
2. General coding-agent requests that should be executed without involving Specd
3. Requests where the user explicitly chooses whether Specd should be used

## Required Workflow Improvements

### 1. Undo and Reopen Workflow

Specd should support an explicit undo or reopen workflow for any lifecycle entity that can validly return to an earlier state.

This includes, where applicable:

* Specifications
* Tasks
* Requirements
* Approvals
* Workflow stages
* Completed work
* Rejected work
* Cancelled work

The analysis must define:

* Which states are eligible for reopening
* Which transitions are valid
* Whether reopening should restore the previous state or create a new revision
* How completed child tasks should be handled
* How reopened work affects specification status
* How reopening affects generated artifacts
* How history and auditability should be preserved
* How invalid state transitions should be prevented
* How the coding agent should behave after work is reopened
* Whether undo and reopen should be separate concepts

Do not treat reopening as a simple status change. Design it as a controlled workflow with clear state-transition rules, revision history, and recovery behavior.

### 2. Pending State

Specd should support a meaningful `pending` state.

Analyze where `pending` is needed and distinguish it from states such as:

* Draft
* Blocked
* Waiting for approval
* Waiting for clarification
* Paused
* Deferred
* Ready
* In progress

Define:

* The semantic meaning of `pending`
* Which entities can become pending
* Valid transitions into and out of pending
* Whether a reason is mandatory
* Whether pending work should block parent completion
* How pending items appear in reports and agent context
* How the coding agent should respond to pending work

Avoid introducing a generic state with unclear meaning. Recommend a precise state model or multiple explicit states if that is more appropriate.

### 3. Configuration File Relocation and Rename

Rename:

```text
project.yaml
```

to:

```text
.specd/config.yaml
```

Analyze and define:

* The new configuration resolution strategy
* Backward compatibility requirements
* Automatic migration behavior
* Deprecation warnings
* Precedence rules if both files exist
* Validation and error handling
* Effects on existing projects
* Effects on CLI commands
* Effects on documentation and examples
* Effects on repository discovery
* Effects on tests, fixtures, and templates
* Whether global and project-level configuration should be supported
* Whether `.specd/config.yaml` should contain only configuration or also workflow metadata

The recommendation must avoid silent configuration ambiguity.

## Main Analysis Tasks

### Phase 1: Understand Existing Problems

Analyze all provided feedback documents and extract:

* Repeated pain points
* Root causes
* Workflow bottlenecks
* Missing abstractions
* Confusing concepts
* Unnecessary steps
* Agent-behavior problems
* Configuration problems
* State-management problems
* Approval-flow problems
* Specification-authoring problems
* Task-execution problems
* Context-loading problems
* Failure-recovery problems
* Human-agent interaction problems
* Areas where Specd becomes intrusive
* Areas where Specd does not provide enough control

Do not only summarize the feedback. Trace each issue to the likely architectural, workflow, interaction, or product-design cause.

### Phase 2: Evaluate the Existing Specd Workflow

Reconstruct the current Specd lifecycle from the repository and feedback documents.

Evaluate:

* Project initialization
* Configuration discovery
* Requirement intake
* Clarification
* Specification generation
* Specification review
* Task generation
* Task execution
* Approval
* Completion
* Reopening
* Failure recovery
* Debugging
* Unattended execution
* Context enforcement
* Interaction with coding agents
* Interaction with normal repository work

For each stage, document:

* Entry conditions
* Expected inputs
* Produced outputs
* State transitions
* User responsibilities
* Coding-agent responsibilities
* Specd responsibilities
* Failure modes
* Recovery paths
* Current friction
* Recommended improvements

### Phase 3: Separate Specd Work from General Agent Work

Design a clear interaction model that prevents every coding request from being forced through Specd.

The analysis must recommend a routing mechanism that can classify requests into modes such as:

* General coding mode
* Specd-managed mode
* Specd consultation mode
* Specd enforcement mode
* Explicit user-selected mode

Consider:

* Explicit command prefixes
* Session-level modes
* Repository-level defaults
* Request classification
* Confidence thresholds
* User confirmation only when genuinely ambiguous
* Bypass commands
* Temporary suspension of Specd
* Read-only Specd consultation
* Rules for when Specd enforcement is mandatory
* How agents should disclose which mode they are using
* How accidental Specd activation should be prevented

The default experience should not add workflow overhead to unrelated development tasks.

### Phase 4: Design the Improved Workflow

Produce a recommended end-to-end workflow that is:

* Easier to configure
* Easier to understand
* Easier to steer
* Less intrusive
* Recoverable
* Auditable
* Suitable for coding agents
* Suitable for human developers
* Safe for unattended execution
* Explicit about approvals
* Explicit about state transitions
* Flexible enough for debugging, greenfield development, and maintenance work

Include a proposed state machine for:

* Specifications
* Tasks
* Approvals
* Execution runs
* Clarification requests

Define valid transitions and invalid transitions.

The workflow should explicitly support:

* Drafting
* Clarification
* Approval
* Execution
* Blocking
* Pausing
* Pending states
* Failure
* Completion
* Reopening
* Cancellation
* Revision
* Superseding outdated specifications

## Required Deliverables

### 1. Workflow Improvement Specification

Create a complete specification describing the improvements required in the Specd workflow.

The specification must include:

* Problem statement
* Background
* Goals
* Non-goals
* User personas
* Core use cases
* Functional requirements
* Non-functional requirements
* Workflow rules
* State-transition rules
* Agent-interaction rules
* Configuration changes
* Migration requirements
* Error-handling requirements
* Auditability requirements
* Compatibility requirements
* Security and safety considerations
* Acceptance criteria
* Open questions
* Risks
* Recommended implementation sequence

Use precise and testable requirements.

### 2. Implementation Tasks

Generate implementation tasks derived from the specification.

Each task must include:

* Title
* Purpose
* Scope
* Dependencies
* Files or components likely affected
* Implementation guidance
* Edge cases
* Testing requirements
* Acceptance criteria
* Migration considerations
* Documentation requirements

Tasks must be ordered by dependency and grouped into implementation phases.

Avoid oversized tasks. Break work into independently reviewable units.

### 3. Domain Analysis Directory

Create a directory containing Markdown documents representing the major domains that need improvement in the Specd project.

Use a structure similar to:

```text
specd-workflow-improvements/
├── README.md
├── workflow-state-management.md
├── undo-and-reopen.md
├── pending-and-blocked-states.md
├── configuration-and-project-discovery.md
├── coding-agent-routing.md
├── specification-authoring.md
├── task-generation-and-execution.md
├── approvals-and-unattended-execution.md
├── context-management-and-enforcement.md
├── debugging-and-failure-recovery.md
├── user-experience-and-steering.md
├── migration-and-backward-compatibility.md
├── testing-and-observability.md
└── implementation-roadmap.md
```

Adjust the domain list when the evidence supports a better decomposition.

Each file must address exactly one domain.

Each domain document must include:

* Domain definition
* Current behavior
* Evidence from the feedback documents
* Main problems
* Root-cause analysis
* Desired behavior
* Recommended design
* Workflow implications
* Data-model implications
* CLI implications
* Coding-agent implications
* Compatibility implications
* Failure scenarios
* Edge cases
* Testing strategy
* Implementation recommendations
* Trade-offs
* Risks
* Acceptance criteria
* Open questions

Do not create shallow summaries. Each domain document should contain complete analysis and concrete recommendations based on software-engineering and workflow-design best practices.

### 4. Directory README

The directory `README.md` must provide:

* Executive summary
* Main findings
* Most critical workflow failures
* Proposed target workflow
* Domain map
* Recommended priorities
* Dependency map
* Suggested implementation phases
* Links to every domain document
* Links to the generated specification and task plan

## Analysis Standards

The work must:

* Be grounded in the provided feedback files
* Distinguish symptoms from root causes
* Avoid proposing features without explaining their workflow impact
* Preserve Specd's core philosophy where it remains valuable
* Identify where the current philosophy creates harmful rigidity
* Recommend changes that reduce cognitive and operational overhead
* Prefer explicit state models over implicit behavior
* Prefer deterministic agent behavior over prompt-only conventions
* Prefer auditable workflow transitions
* Preserve user control
* Prevent coding agents from falsely claiming completion
* Prevent agents from bypassing required Specd rules when Specd mode is active
* Prevent Specd from interfering when Specd mode is inactive
* Consider both interactive and unattended workflows
* Include backward-compatible migration strategies
* Produce implementation-ready recommendations

## Important Constraints

* Do not modify the repository before completing the analysis and proposed design.
* Do not assume that every issue should be solved through prompting.
* Identify where changes are required in architecture, CLI behavior, data models, state machines, configuration, documentation, and agent integration.
* Do not hide unresolved design questions.
* Do not generate generic best-practice advice disconnected from the repository.
* Do not collapse all findings into one large document.
* Every domain must have a dedicated Markdown file.
* Every recommendation must explain why it is needed and how it should be implemented.
* Requirements and acceptance criteria must be testable.
* Preserve a clear distinction between Specd-managed work and ordinary coding-agent work.

## Final Output

Produce:

```text
specd-workflow-improvements/
├── README.md
├── specification.md
├── implementation-tasks.md
├── workflow-state-management.md
├── undo-and-reopen.md
├── pending-and-blocked-states.md
├── configuration-and-project-discovery.md
├── coding-agent-routing.md
├── specification-authoring.md
├── task-generation-and-execution.md
├── approvals-and-unattended-execution.md
├── context-management-and-enforcement.md
├── debugging-and-failure-recovery.md
├── user-experience-and-steering.md
├── migration-and-backward-compatibility.md
├── testing-and-observability.md
└── implementation-roadmap.md
```

The exact structure may be improved when justified by the analysis, but the final output must contain:

1. One complete workflow-improvement specification
2. One dependency-ordered implementation task plan
3. One Markdown document per improvement domain
4. One README connecting all findings and documents
5. A concrete proposal for separating normal coding-agent requests from Specd-managed requests
6. A complete undo and reopen workflow
7. A precise pending-state model
8. A migration plan from `project.yaml` to `.specd/config.yaml`
