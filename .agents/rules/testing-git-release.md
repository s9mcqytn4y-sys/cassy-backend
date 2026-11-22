---
trigger: model_decision
description: "Aktif saat menambah test, melakukan review, commit, membuat release, atau deployment."
---

# Testing, Git, and release rules

- Follow the repository's actual branch and CI policy. The flow is short-lived branch -> PR to `main` -> demo -> owner-approved `v*` tag for production; do not assume branch names exist before checking Git.
- Before editing, inspect `git status` and preserve unrelated user changes.
- Never use destructive Git commands such as `reset --hard`, force push, or branch deletion unless the owner explicitly approves.
- Run focused tests first, then the relevant broader test/build checks. Report commands and results accurately.
- For Go: run `gofmt -l .` (must return 0 files), `go vet ./...`, and `go test -race ./...` before pushing. CI workflows must read the Go version via `go-version-file: 'go.mod'`. For a separate Next.js repo: use its lockfile/package manager and run lint, typecheck, tests, and production build as configured.
- Secret & dummy token hygiene: never commit high-entropy mock tokens (e.g. `tk-xxx-yyyy`) in fixtures, HTML templates, or docs that trigger `gitleaks`; use semantic identifiers like `demo-order-14`.
- Do not use mutable action tags or unreviewed third-party GitHub Actions in production deployment workflows. Pin actions to reviewed commit SHAs where practical.
- Production deployment requires a reviewed release, known environment variables, health checks, a rollback plan, and explicit authorization.
- Do not use a GitHub Actions scheduled workflow as the only backup mechanism for a production database without a tested backup/restore design.
