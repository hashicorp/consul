# AGENTS.md

Context file for AI agents working on consul.

**Dual Format**: This file combines Category A (Operations Manual) and Category B (Context Guide) for comprehensive agent guidance.

## Project Overview

consul is a Go project using Go (Makefile).

**Key Info:**
- **Primary Language:** Go
- **Build System:** Go (Makefile)
- **Test Framework:** Go testing
- **Total Files:** 11402
- **Test Files:** 2329
- **AI Readiness Score:** 98/100 (Agent-Optimized)

---

## 🚨 AI Policy & Operations

Extracted from CONTRIBUTING.md - operational constraints and procedures.

### AI Policy

- rules to get in the way of that.
- If you make any changes to the code, run `gofmt -s -w` to automatically format the code according to Go standards.
- `go test -v -run TestRetryJoin ./command/agent` will run all tests in the agent package (see `./command/agent` folder) with name substring `TestRetryJoin`
- | `backport/1.12.x`    | Backport the changes in this PR to the targeted release branch. Consult the [Consul Release Notes](https://developer.hashicorp.com/docs/release-notes) page and [`versions.hcl`](/.release/versions.hcl) to view active releases. Website documentation merged to the latest release branch is deployed immediately. See [backport policy](#backport-policy) for more information. |
- | `backport/all`       | If contributing a bug fix or other change applicable to all branches, use `backport/all` to target all active branches automatically. See [backport policy](#backport-policy) for more information. |

### Key Requirements

- linked. Any change a Consul user might need to know about will include a
- If you wish to work on Consul itself, you'll first need to:
- maintainers can provide their perspective if needed.
- If there's anything you find the need to explain or clarify in the PR, consider
- 5. If there's any reason Consul users might need to know about this change,

### Development Procedures

- Increase our test coverage.
- Make sure you test against the latest released version. It is possible we
- Provide a reproducible test case. If a contributor can't reproduce an issue,
- 4. The issue is addressed in a pull request or commit. The issue will be
- referenced in the commit message so that the code that fixes it is clearly



## 🏗️ Architecture & Context Guide

This section provides architectural context and agent-understanding for the codebase.

### Prerequisites

- **Go:** 1.18+ (or applicable language version)
- **Package Manager:** go modules
- **Test Runner:** Go testing

### Environment Requirements

- **Go:** 1.26.7+ (from `go.mod`)
  - GCC required for CGo/SQLite compilation
- **Package Manager:** go modules


### Project Structure

```
consul/
├── Makefile
├── package.json
├── package.json
├── src/                  # Source code
├── tests/                # Test suite (2329 files)
└── README.md             # Project documentation
```

### Architecture Overview

#### Key Components
- **Main Entry:** main.go, index.js, index.js, index.js, index.js
- **Test Suite:** 2329 test files
- **Build Configuration:** Makefile, package.json, package.json

#### Design Principles

1. **Modularity** - Code organized by functionality with clear separation of concerns
2. **Testability** - Comprehensive test coverage across critical paths
3. **Clarity** - Explicit naming and structure for AI agent understanding
4. **Consistency** - Uniform patterns and conventions throughout codebase
5. **Maintainability** - Well-documented code with clear intent

### Directory Map

| Directory | Purpose |
|-----------|----------|
| `api/` | API handlers |
| `docs/` | Documentation |
| `lib/` | Library code |
| `test/` | Test suite |


### Development Workflow

#### Initial Setup

```bash
git clone https://github.com/YOUR_ORG/consul.git
cd consul
go mod download
```

#### Development Commands

**Running Tests:**
```bash
go build ./...            # Build project
go test ./...             # Run all tests
go test -v ./...          # Verbose test output
golangci-lint run         # Lint (if installed)
```

#### Code Quality
```bash
gofmt -w .                # Format code
go vet ./...              # Vet (static analysis)
```

### Code Style & Conventions

- **Naming:** Use Go conventions (snake_case for functions, PascalCase for classes)
- **Type Hints:** Yes (strongly encouraged)
- **Error Handling:** Yes - handle errors at boundaries; let exceptions propagate when another layer owns recovery
- **Logging:** Yes
- **Testing:** Yes - write tests alongside code changes

### Testing Strategy

**Framework:** Go testing
**Test Files:** 2329 found

Before committing:
1. Run the full test suite: `go test ./...`
2. Ensure all tests pass: `go test -v ./...`
3. Run linter: `golangci-lint run`
4. Format code: `gofmt -w .`

### Writing Documentation

When updating docs:
1. Always include explanatory text before code snippets
2. Describe *why* and *what* before showing *how*
3. Keep sections focused on a single concept
4. Use clear, concrete examples

## Known Gotchas & Warnings

- **Note:** We take Consul's security and our users' trust very seriously.
- something. We appreciate any sort of contributions, and don't want a wall of
- Note: Issues on GitHub for Consul are intended to be related to bugs or feature requests.
- Note: `make dev` will build for your local machine's os/architecture. If you wish to build for all os/architecture combinations, use `make`.

### Contributing Guidelines

This project has a detailed contribution guide at **`.github/CONTRIBUTING.md`**.

**Key Requirements:**
- **Release Notes Block**: Include `release-notes` block in every PR description

**Before submitting:**
1. Read `.github/CONTRIBUTING.md` in full
2. Check recent merged PRs for patterns
3. Follow the specific requirements above

### Common Patterns

When contributing to this project:
1. Read existing code in the area you're modifying
2. Follow the established patterns and style
3. Write tests for new functionality
4. Use clear, descriptive variable and function names
5. Add docstrings for public APIs
6. Update tests when changing behavior

### What We Value

✅ Well-tested code with clear intent
✅ Consistent code style and naming conventions
✅ Code that is easy for AI agents to understand
✅ Clear, descriptive commit messages
✅ Modular, reusable components
✅ Comprehensive documentation

### What We Avoid

❌ Large functions doing multiple things
❌ Commented-out dead code
❌ Inconsistent naming or patterns
❌ Unclear error messages
❌ Unexplained magic numbers or strings
❌ Skipped tests or test TODOs

### AI Readiness Dimensions (Scoring)

This project is evaluated across 8 dimensions:

1. **Architecture** (20/100) - Code organization and modularity
2. **Testing** (15/100) - Test coverage and quality
3. **Dependencies** (12/100) - Dependency management
4. **Conventions** (8/100) - Consistent patterns
5. **Entry Points** (10/100) - Clear main/start locations
6. **Security** (15/100) - Input validation and error handling
7. **Build** (10/100) - Clear build/setup instructions
8. **Documentation** (8/100) - Code and project documentation

### Next Steps

Before making changes:
1. Read relevant source files to understand the existing code
2. Look at existing tests for similar functionality
3. Follow the patterns you see in the codebase
4. Write tests for your changes
5. Run `pytest` to verify nothing breaks
6. Run code quality checks: `ruff check . && mypy .`
7. Format your code: `ruff format .`

---

*Generated by Braxis - keeping AI agents in sync with your code*

