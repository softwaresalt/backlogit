<!-- markdownlint-disable-file -->
# PR Review Handoff: 013-release-pipeline-fix

## PR Overview

This branch finishes **F013: Release Pipeline Fix** and also hardens the review
and PR control plane that was used to deliver it.

The branch now includes:

* workflow security and reliability fixes in `.github/workflows/`
* an integration harness that keeps those workflow invariants enforced
* first-class `review` artifact support in backlogit
* review and orchestration instruction hardening across agents and skills
* cleanup for generated or tracked review artifacts that should not ship

* Branch: `013-release-pipeline-fix`
* Base Branch: `main`
* Head Commit: `1e4d405`
* Pull Request: `#6` <https://github.com/softwaresalt/backlogit/pull/6>
* Total Files Changed: 60
* Backlogit Work Item: `F013`
* Review Artifacts:
  * `.backlogit/queue/F013.R001-branch-review.md`
  * `.backlogit/queue/F013.R002-followup-review.md`

## PR Title

`fix(backlogit): release pipeline and review hardening`

## PR Body

```markdown
## Summary

Completes F013 by fixing the release-pipeline defects that blocked compliant
GitHub Actions execution, then hardens the review and PR control plane that
shipped those fixes.

## Changes

### Release pipeline compliance

- fix the release tag trigger to use GitHub's glob syntax
- move workflow Go versions to the 1.23/1.24 line required by `go.mod`
- pin third-party workflow actions to full SHAs
- add `persist-credentials: false` to checkout steps
- add explicit `concurrency` blocks
- scope release write permissions to the release job
- pin `golangci-lint` instead of using `latest`

### Validation coverage

- add `tests/integration/ci_compliance_test.go` to lock in the workflow
  invariants
- promote the YAML and validator packages used by the test harness to direct
  dependencies

### Review artifact support

- add first-class `review` artifact support with the `R` prefix
- support stable review IDs with descriptive filenames such as
  `F013.R001-branch-review.md`
- enforce valid parentage for review artifacts
- update archive and metadata coverage for the new artifact type

### Review and agent hardening

- make review `report-only` behavior truly side-effect free
- move plan-review tracking output to `.copilot-tracking/plan-review/`
- narrow researcher and reviewer tool permissions
- harden commit-link guidance so commits track only directly affected items
- strengthen `.github/copilot-review-instructions.md` for adversarial,
  multi-pass review

### Branch hygiene

- stop shipping generated PR reference XML files
- remove tracked SQLite sidecar churn from the branch
- drop unrelated stale PR review tracking artifacts
- keep the durable F013 memory and compound docs focused on long-lived outcomes

## Validation

- `go test ./...`
- `go vet ./...`
- `golangci-lint run`

## Review artifacts

- Original review: `F013.R001`
- Follow-up review: `F013.R002`

## Follow-up

- Deferred stash item: `2ED12470`
```

## Review Outcome

All blocking review findings are resolved. The remaining follow-up is the
advisory stash item `2ED12470` for stricter type assertions in the CI
compliance tests.
