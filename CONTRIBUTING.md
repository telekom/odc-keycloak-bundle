# Contributing to keycloak-bundle

## Branching Strategy

| Branch pattern | Purpose |
|----------------|---------|
| `main` | Stable delivery branch. Direct commits are not permitted. |
| `feature/<REQ-ID>-<short-description>` | New feature or requirement implementation. |
| `fix/<short-description>` | Bug fix or hotfix. |
| `delivery/<milestone>` | Release preparation (docs, changelog, final integration). |

Branch off `main`, open a pull request back to `main`. Delete the branch after merge.

## Commit Message Conventions

Use the following prefixes:

```
feat(<scope>): short description
fix(<scope>):  short description
docs:          short description
ci:            short description
refactor:      short description
test:          short description
```

**Rules:**
- Subject line: 72 characters maximum, imperative mood, no trailing period.
- Scope is optional but recommended for `feat` and `fix` (e.g. `operator`, `backup`, `deploy`).
- Body is optional; use it to explain *why*, not *what*.
- Do not reference ticket numbers in commit messages; use the branch name for traceability.

## Pull Request Review Process

1. Open the PR against `main` with a title following the commit conventions above.
2. The PR description must summarise the change and list any deferred or untested items.
3. At least one reviewer approval is required before merge.
4. All CI quality gates (see below) must pass before merge is permitted.
5. Squash merge is preferred to keep `main` history linear.
6. The author resolves review comments; the reviewer approves the resolution.

## CI Quality Gates

Every PR must pass the following checks before it can be merged. These run automatically
on push and pull request events via the `OCM Build & Deploy` workflow and `Security Scan`
workflow.

| Gate | Tool | Failure condition |
|------|------|-------------------|
| YAML Lint | yamllint | Any syntax error or rule violation in `manifests/`, `kro/`, `charts/`, `examples/` |
| Shell Lint | ShellCheck `-S warning` | Any warning or error in `scripts/**/*.sh` |
| Secrets Scan | Gitleaks | Any credential or secret pattern detected in the git history |
| Go Vet | `go vet ./...` | Any static analysis error in `operator/` |
| Non-root Check | `scripts/tests/test-security.sh` | Any Deployment manifest missing `allowPrivilegeEscalation: false`, `runAsNonRoot: true`, or `capabilities.drop: [ALL]` |
| CVE Scan | Trivy | Any unfixed HIGH or CRITICAL CVE in a container image shipped in the OCM archive |

**Bypass is not permitted.** If a gate fails, fix the underlying issue — do not skip or
disable the gate.

## Development Workflow

```bash
# 1. Create a branch
git checkout main && git pull
git checkout -b feature/REQ-XX-short-description

# 2. Make changes, commit, push
git add <files>
git commit -m "feat(scope): describe the change"
git push -u origin feature/REQ-XX-short-description

# 3. Open a pull request via GitHub
# 4. Address review comments, push additional commits
# 5. Merge after approval and all gates green
```

## Local Quality Checks

Run the same checks locally before pushing to avoid CI failures:

```bash
# YAML lint
yamllint -d '{extends: default, rules: {line-length: {max: 150}, truthy: {check-keys: false}, document-start: disable, comments-indentation: disable}}' manifests/ kro/ charts/ examples/

# ShellCheck
shellcheck -S warning scripts/**/*.sh

# Go vet
cd operator && go vet ./...

# Non-root security check
./scripts/tests/test-security.sh

# Secrets scan (requires gitleaks in PATH)
gitleaks git .
```
