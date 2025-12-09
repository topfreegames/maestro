# Execution Workflow Template

Use this template for every request to ensure consistent, high-quality execution.

## Step 1: Memory Bank Loading ✅
- [ ] Read `docs/memory-bank/system-patterns.md`
- [ ] Read `docs/memory-bank/tech-context.md`
- [ ] Context fully loaded and understood

## Step 2: Deep Understanding & Clarification

### Request Analysis
**Original Request:**
```
[Copy the user's exact request here]
```

**My Understanding:**
```
[Explain in detail what you understand the user is asking for]
```

**Clarifying Questions (if any):**
```
[List any questions needed to fully understand the request]
```

**Final Understanding Statement:**
```
[Clear, explicit statement of what will be implemented]
```

**Confirmation:** Should we proceed to create the execution plan?

## Step 3: Execution Plan Creation

### Phase Breakdown

#### Phase 1: [Phase Name]
**Deliverables:**
- [ ] [Specific deliverable 1]
- [ ] [Specific deliverable 2]

**Test Cases:**
- [ ] Unit tests: [describe tests]
- [ ] Integration tests: [describe tests]
- [ ] Manual verification: [describe verification steps]

**Interface Testing Coverage (CRITICAL for interface implementations):**
- [ ] All interface methods identified: [list all methods]
- [ ] Complex methods prioritized
- [ ] Success scenarios tested for all methods
- [ ] Error scenarios tested for all methods
- [ ] Dependencies mocked properly
- [ ] Business logic verified, not just method signatures

**Quality Checks:**
- [ ] Linting: `make lint/go` or `golangci-lint run`
- [ ] Formatting: `gofmt -s -w .`
- [ ] Compilation: `go build ./...`
- [ ] Tests: `go test ./...`

#### Phase 2: [Phase Name]
**Deliverables:**
- [ ] [Specific deliverable 1]
- [ ] [Specific deliverable 2]

**Test Cases:**
- [ ] Unit tests: [describe tests]
- [ ] Integration tests: [describe tests]
- [ ] Manual verification: [describe verification steps]

**Interface Testing Coverage (CRITICAL for interface implementations):**
- [ ] All interface methods identified: [list all methods]
- [ ] Complex methods prioritized
- [ ] Success scenarios tested for all methods
- [ ] Error scenarios tested for all methods
- [ ] Dependencies mocked properly
- [ ] Business logic verified, not just method signatures

**Quality Checks:**
- [ ] Linting: `make lint/go` or `golangci-lint run`
- [ ] Formatting: `gofmt -s -w .`
- [ ] Compilation: `go build ./...`
- [ ] Tests: `go test ./...`

#### Phase N: [Final Phase]
**Deliverables:**
- [ ] [Final deliverable]
- [ ] Documentation updates
- [ ] Integration verification

**Test Cases:**
- [ ] End-to-end testing
- [ ] Performance verification
- [ ] Security validation

**Quality Checks:**
- [ ] Final linting validation
- [ ] Complete compilation check
- [ ] Full test suite execution
- [ ] Code review checklist

### Risk Assessment
**Potential Risks:**
- [ ] [Risk 1 and mitigation strategy]
- [ ] [Risk 2 and mitigation strategy]

**Dependencies:**
- [ ] [External dependency 1]
- [ ] [External dependency 2]

**Confirmation:** Should we proceed with implementation?

## Step 4: Plan Review & Approval

**User Feedback:**
```
[Record any feedback or modifications requested]
```

**Plan Adjustments:**
```
[Document any changes made to the plan based on feedback]
```

**Final Approval:** ✅ Confirmed to proceed with implementation

## Step 5: Implementation Progress Tracking

### Phase 1: [Phase Name] - Status: [Not Started/In Progress/Completed]
- [ ] **Implementation:** [Implementation details]
- [ ] **Tests Executed:** [Test results]
- [ ] **Linting:** ✅ Passed / ❌ Failed - [Details]
- [ ] **Compilation:** ✅ Passed / ❌ Failed - [Details]
- [ ] **Phase Complete:** ✅ / ❌

**Notes:**
```
[Any important notes, issues encountered, or decisions made]
```

### Phase 2: [Phase Name] - Status: [Not Started/In Progress/Completed]
- [ ] **Implementation:** [Implementation details]
- [ ] **Tests Executed:** [Test results]
- [ ] **Linting:** ✅ Passed / ❌ Failed - [Details]
- [ ] **Compilation:** ✅ Passed / ❌ Failed - [Details]
- [ ] **Phase Complete:** ✅ / ❌

**Notes:**
```
[Any important notes, issues encountered, or decisions made]
```

### Phase N: [Final Phase] - Status: [Not Started/In Progress/Completed]
- [ ] **Implementation:** [Implementation details]
- [ ] **Tests Executed:** [Test results]
- [ ] **Linting:** ✅ Passed / ❌ Failed - [Details]
- [ ] **Compilation:** ✅ Passed / ❌ Failed - [Details]
- [ ] **Phase Complete:** ✅ / ❌

**Notes:**
```
[Any important notes, issues encountered, or decisions made]
```

## Final Verification

### Overall Project Status
- [ ] All phases completed successfully
- [ ] All tests passing
- [ ] No linting errors
- [ ] Successful compilation
- [ ] Documentation updated
- [ ] Ready for review/deployment

### Summary
**What was accomplished:**
```
[Summary of all work completed]
```

**Files modified/created:**
```
[List of all files that were changed or created]
```

**Tests added/modified:**
```
[List of test files and test cases]
```

**Verification commands:**
```bash
# Commands to verify the implementation works
make run/tests
make lint
make build
# Any additional verification steps
```

**Next steps (if any):**
```
[Any follow-up work or recommendations]
```

---

## Template Usage Notes

1. **Copy this template** for each new request
2. **Fill in each section** as you progress through the workflow
3. **Update progress in real-time** to maintain context
4. **Use checkboxes** to track completion status
5. **Document everything** - assume the agent might be interrupted
6. **Be specific** - avoid vague descriptions
7. **Include commands** - provide exact commands for verification
