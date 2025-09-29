CONSUL PR REVIEW – COMPACT AI INSTRUCTIONS

GOAL
High-signal automated review. Surface only material issues. Categorize by severity: MUST (blocks), SHOULD (recommended), INFO (non-blocking).

WORKFLOW (ALWAYS IN THIS ORDER)
1 Classify diff scope (code / tests / docs / config / proto / UI / deps / build).
2 Select only relevant rule categories.
3 Scan diff (not whole repo) for violations.
4 Produce summary (counts) then focused actionable comments.
5 Combine repeated pattern instances into one comment with examples.

OUTPUT FORMAT
Summary: BLOCKERS: X | SHOULD: Y | INFO: Z
Then issues grouped by category. Each:
[RULE-ID] One-line problem
Fix: Concrete minimal remediation

PROHIBITED
- No abandoned architecture references.
- No speculative refactors or style nits without rule ID.
- No invented config/flags.
- Don’t suggest API renames without deprecation path.

CORE MUST RULES (retain only most impactful)
DOC-001 Grammar and punctuation correct in comments, changelog, user-visible errors.
DOC-003 Exported identifiers have godoc starting with the identifier.
DOC-006 SPDX header in new Go files.
GO-001 No ignored errors.
GO-002 Use fmt.Errorf with %w (no string concat wrapping).
GO-003 Context first param; not stored globally.
GO-004 No naked returns (>5 line funcs).
GO-005 Struct literals (>3 fields) keyed.
GO-008 No panics except unrecoverable init.
GO-010 No var shadowing hiding errors.
DEP-002 No unused imports/vars; go mod tidy clean.
DEP-005 go.mod / go.sum must be tidy (no drift).
TEST-001 New logic has unit tests (happy + failure path).
TEST-004 No hard sleeps; use polling/timeout.
TEST-007 Golden file updates intentional for xDS changes.
CONSUL-001 ACL check before state mutation.
CONSUL-002 HTTP input validated; consistent error JSON.
CONSUL-004 Use existing HCL validation (no duplicate parsers).
CONSUL-006 RPC endpoints respect context cancel / DC forwarding.
SEC-001 Never log secrets / tokens / private material.
SEC-002 Validate all external inputs (length/charset/range).
SEC-003 Modern TLS only (no deprecated ciphers/protos).
SEC-005 crypto/rand for entropy-critical values.
PERF-001 No unbounded O(n^2) over cluster/member lists.
PERF-003 Close all bodies/files/connections (no leaks).
ERR-001 Wrap & contextualize propagated errors (%w).
ERR-005 Stop on unrecoverable error; no partial silent continuation.
CONC-001 Protect shared mutable state (locks/ownership).
CONC-002 Goroutines have termination path (ctx.Done()).
CONC-004 Don’t hold locks across network/disk I/O.
CONC-007/008 Channels not double-closed / written after close.
LOGIC-001 Bounds check before indexing with external/user data.
LOGIC-002 No off-by-one (avoid i <= len()).
LOGIC-005 Multi-step mutation has rollback or is idempotent.
LOGIC-006 Retries use max attempts + backoff/jitter.
LOGIC-008 No silent error swallowing.
CONF-001 New config fields: default + validation + docs + changelog (unless pr/no-changelog).
CONF-002 Field removal/rename requires deprecation path.
API-001 Public API: backward compatible or deprecation noted.
API-003 Proto: no tag reuse / renumbering; removed tags reserved.
METRIC-001 Metrics avoid high-cardinality labels; stable names.
UI-001 Interactive elements keyboard accessible.
UI-004 Significant UI behavior changes include/update acceptance or page object tests.
UPG-001 Persistent/on-disk/raft format changes include migration or explicit rationale.
GUARD-002 Generated code not manually edited (proto, mocks) unless regenerated.
GUARD-003 Remove debug println/log.Printf (use structured logger).

SHOULD (mention only if diff touches them & easy to fix)
GO-007 Reduce deep nesting with early returns.
TEST-003 Use testify/suite for complex multi-step state.
PERF-002 Cache repeated expensive lookups with invalidation.
LOGIC-007 Explicitly reject unknown enum/config values.
METRIC-002 Add units comment if not obvious.
UI-003 Prefer existing design system components.
UPG-003 Note mixed-version behavior if relevant.

LOGICAL ERROR HEURISTICS (MUST when present)
LE-BOUNDS Slice/map index or len math must guard against nil/short slices.
LE-NIL Nil ptr from map lookup or type assertion checked before method call.
LE-DEFER Defer unlock/close immediately after acquire/open; no missing paths.
LE-RETRY Retry loops bounded; backoff used (no tight infinite loop).
LE-RACE Unsynchronized read/write to map/slice in goroutines rejected.
LE-RESOURCE File/conn/goroutine started must have deterministic close/stop.
LE-SHADOW Inner := must not hide outer err causing skip of handling.
LE-TIMEOUT External calls use context with timeout/cancel.

AUTO-FIX HINT KEYS (include when obvious): WrapError, AddGodoc, ImportOrder, AddBoundsCheck, AddContextParam, CloseResource, AddRetryBackoff, ReplacePanic.

SUMMARY TEMPLATE
BLOCKERS: <n> | SHOULD: <n> | INFO: <n>
Categories: (list affected prefixes)
Example:
[GO-002] String concat used for wrapping err in pkg/foo/bar.go:57
Fix: return fmt.Errorf("opening index %s: %w", name, err)

APPROVAL CHECKLIST
1 No remaining MUST violations.
2 Security + data integrity risks addressed.
3 Tests updated/added for new logic & failures.
4 Changelog/config/docs updated where required.
5 No secret or debug output.
6 Concurrency safe (locks, ctx cancel, no leaks).
7 API / proto compatibility preserved or deprecated properly.

END GOAL