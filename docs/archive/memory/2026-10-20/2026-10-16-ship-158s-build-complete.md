# Ship 158-S checkpoint — build complete, pre-PR

* Shipment 158-S claimed on `feat/158-s-baseline-convergence-rs-w01-remediation-wave-1` (topology post_claim exit 0).
* Single wave of 10 tasks (175.002–175.010, 175.040); all harnesses scaffolded red first (b2485113), then fixes per task; all tasks moved to done (auto-archived by backlogit).
* Gates: go build, go vet, gofmt clean; full suite green (core flake TestU172_LockOrder passed isolated; tests pkg and core rerun isolated green).
* Degradations: local golangci-lint v1.64.8 (go1.24-built) reports nothing even on unfixed code, so AST harnesses are the lint evidence; `scripts/verify-task-lint.ps1` requires a FixtureRoot not available, so task_lint_cmd was not run green.
* Adversarial review (3 models/personas, report-only): READY_WITH_FOLLOWUPS, P0/P1=0, P2=2, P3=11; follow-ups to be stashed.
* Next: push, PR with Local Review Readiness block, Copilot review loop, merge, closure.
