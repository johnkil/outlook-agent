# Phase 76 FindFolder Bounded Decision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:systematic-debugging and superpowers:test-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the `FindFolder` production gate as a bounded compatibility decision after six sanitized metadata-only live candidates returned the same OWA internal error.

**Architecture:** Keep `FindFolder` classified and available through guarded raw execution, but stop treating this deployment-specific live payload gap as an open repository-owned gate. Guard the decision with a documentation test, update readiness/backlog/action-coverage docs, close the GitHub issue with public-safe evidence, and keep private endpoint/session material out of the repository.

**Tech Stack:** Markdown docs, Go documentation tests, GitHub CLI.

---

### Task 1: Guard The Bounded Decision

**Files:**
- Modify: `internal/app/production_readiness_doc_test.go`
- Modify: `docs/PRODUCTION_BACKLOG.md`
- Modify: `docs/PRODUCTION_READINESS.md`
- Modify: `docs/ACTION_COVERAGE.md`
- Modify: workspace spike log outside this public repository

- [x] **Step 1: Write the failing documentation test**

Add `TestProductionBacklogBoundsFindFolderCompatibilityDecision` that:

```go
func TestProductionBacklogBoundsFindFolderCompatibilityDecision(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "PRODUCTION_BACKLOG.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read production backlog: %v", err)
	}
	text := string(data)

	required := []string{
		"## Bounded Compatibility Decisions",
		"FindFolder compatibility",
		"https://github.com/johnkil/outlook-agent/issues/7",
		"six metadata-only candidates",
		"ErrorInternalServerError",
		"does not expose a compatible metadata-only `FindFolder` shape",
		"guarded raw action transport",
	}
	for _, marker := range required {
		if !strings.Contains(text, marker) {
			t.Fatalf("expected production backlog to contain %q", marker)
		}
	}

	openSection := sectionBetween(text, "## Open External Gates", "## Bounded Compatibility Decisions")
	if strings.Contains(openSection, "FindFolder compatibility follow-up") {
		t.Fatal("FindFolder must not remain in the open external gates table after the bounded decision")
	}
}
```

Add a small `sectionBetween` helper in the test file.

- [x] **Step 2: Run RED**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run TestProductionBacklogBoundsFindFolderCompatibilityDecision -count=1
```

Expected: FAIL because `docs/PRODUCTION_BACKLOG.md` does not yet contain `## Bounded Compatibility Decisions`.

- [x] **Step 3: Update backlog and readiness docs**

Move `FindFolder compatibility follow-up` out of the `Open External Gates` table and add a `## Bounded Compatibility Decisions` section that records:

- issue `#7`;
- six metadata-only candidates;
- the repeated sanitized `ErrorInternalServerError`;
- decision: this deployment does not expose a compatible metadata-only `FindFolder` shape through the tested OWA JSON/URLPostData routes;
- `FindFolder` remains classified and available as a guarded raw action;
- this is not evidence against the guarded raw action transport.

Update `docs/PRODUCTION_READINESS.md` so the live verification row and remaining gaps no longer describe `FindFolder` as an open live-validation gap. Update `docs/ACTION_COVERAGE.md` so the compatibility note uses bounded-decision language.

- [x] **Step 4: Run GREEN**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/app -run 'TestProductionBacklog(TracksExternalGates|BoundsFindFolderCompatibilityDecision)' -count=1
```

Expected: PASS.

- [x] **Step 5: Run verification**

```bash
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/ci-local.sh
GOPATH=$PWD/.cache/go GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod scripts/release-smoke.sh
git diff --check
bash scripts/public-safety-check.sh
```

Also run the parent workspace private-marker grep and temporary artifact check before publishing.

- [x] **Step 6: Close issue and commit**

Close GitHub issue `#7` with a public-safe summary of the bounded decision, then commit:

```bash
git add internal/app/production_readiness_doc_test.go docs/PRODUCTION_BACKLOG.md docs/PRODUCTION_READINESS.md docs/ACTION_COVERAGE.md docs/superpowers/plans/2026-05-28-phase-76-findfolder-bounded-decision.md
git commit -m "docs: bound findfolder compatibility decision"
```
