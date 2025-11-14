# Memory Bank Index

This directory contains the memory bank documentation for the Maestro project. These
files serve as the persistent memory for AI agents working on this project.

## 🚨 MANDATORY EXECUTION WORKFLOW

**EVERY AI AGENT MUST FOLLOW THIS WORKFLOW FOR ALL REQUESTS:**

### Step 1: Memory Bank Loading
- **ALWAYS** read ALL memory bank files at the start of EVERY task
- Load context from: `system-patterns.md`
- This is **NOT OPTIONAL** - memory resets completely between sessions

### Step 2: Deep Understanding & Clarification
- **2.a** Understand the request profoundly, ask clarifying questions if needed
- **2.b** State explicitly what was understood from the request
- **2.c** Ask: "Should we proceed to create the execution plan?"

### Step 3: Execution Plan Creation
- **3.a** Break down the work into small, testable phases
- For each phase, define:
  - **Specific deliverables**
  - **Test cases** (unit, integration, manual verification)
  - **Linting validation** (`golangci-lint`, `gofmt`)
  - **Compilation verification** (`go build`, `go test`)
- **3.b** Present the complete plan and ask: "Should we proceed with implementation?"

### Step 4: Plan Review & Approval
- Wait for explicit approval before starting implementation
- Address any feedback or modifications to the plan
- Only proceed after clear confirmation

### Step 5: Implementation with Progress Tracking
- **5.a** Execute each phase sequentially. If the phase includes writing new tests, be sure to ask for feedback and approval on implementation before proceeding to writing tests.
- For each completed phase:
  - ✅ Run all defined tests
  - ✅ Validate linting (`make lint/go` or equivalent)
  - ✅ Ensure compilation (`go build ./...`)
  - ✅ Update progress checklist in the plan
- Maintain detailed progress tracking for context preservation if agent is interrupted

### File Interdependencies
To maximize effectiveness:
- Start with `README.md` for workflow overview.
- Load `system-patterns.md` for architectural context.
- Use `execution-workflow-template.md` for task execution guidance.
This sequence ensures full context before planning or implementing changes.

## Core Files

### 🏗️ [system-patterns.md](system-patterns.md)
Documents the Maestro system architecture, design patterns, component relationships,
and critical implementation paths.

### 📝 [execution-workflow-template.md](execution-workflow-template.md)
Template for structuring execution workflow - copy and use for every request to ensure
consistent, high-quality delivery.

## Quick Reference - Maestro

- **Starting ANY task?** Use `execution-workflow-template.md` to structure your approach
- **Need Maestro system patterns?** Check `system-patterns.md` for layered architecture
and Maestro-specific patterns

## Navigation Flow - Maestro Backend

```
1. system-patterns.md (How is Maestro architected?)
   ↓
2. execution-workflow-template.md (How do we execute tasks?)
```

## Maestro-Specific Execution Workflow

```
🔄 EVERY REQUEST MUST FOLLOW THIS FLOW:

1. 📚 Load Memory Bank (ALL files)
   ↓
2. 🎾 Deep Understanding
   ├── Clarify Maestro feature requirements
   ├── State Maestro understanding explicitly
   └── Ask: "Proceed to Maestro planning?"
   ↓
3. 📋 Create Execution Plan
   ├── Break into small testable phases
   ├── Define unit and integration tests
   ├── Include linting & compilation
   └── Ask: "Proceed with Maestro implementation?"
   ↓
4. ✅ Get Plan Approval
   ├── Address feedback
   ├── Validate architecture compliance
   └── Wait for explicit confirmation
   └── Create the task progress file
   ↓
5. 🚀 Implementation with Tracking
   ├── Execute phase sequentially
   ├── Run tests, lint, compile for each phase
   ├── Validate Maestro patterns compliance
   ├── Update progress checklist
   └── Maintain context for interruptions
```
**Important**: Remember, After any memory reset, start by reading ALL Maestro memory
bank files to restore full Maestro Backend project context.
