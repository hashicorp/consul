# Consul UI accessibility audit

Tracks a systematic a11y audit of consul-ui, broken into independent, per-section
tasks. **Audit only** — findings are documented here with suggested fixes, but
no fixes are applied as part of this work.

## Method (applies to every task file)

- **Automated:** two independent rule engines are run against each representative
  page/state, since they catch different things:
  - Chrome DevTools Lighthouse accessibility audit (axe-core-based).
  - IBM's `accessibility-checker` (the open-source engine behind DevTools'
    "Accessibility Assessment" panel — that panel and "Screen Reader Simulator"
    themselves aren't drivable by any available automation tool, since they're
    proprietary Chrome-extension UI with no exposed API; running the engine
    directly via its Node API is the closest equivalent). Installed standalone
    in the scratchpad dir (`npm install accessibility-checker`), *not* added
    as a project dependency. Run via a small script calling
    `aChecker.getCompliance(url, label)` against the dev server.
  - In this pass IBM's checker surfaced 3 concrete bugs axe/Lighthouse missed
    entirely (findings #5–#7 in [01-shared-chrome.md](01-shared-chrome.md)),
    plus independently confirmed the contrast finding (#1) both tools agree on.
    Worth running both engines on every task, not just one.

  **Reproducing the IBM scan** (scratchpad install, not a project dependency —
  recreate this each session, it doesn't persist):
  ```bash
  mkdir a11y-ibm && cd a11y-ibm && npm init -y && npm install accessibility-checker
  ```
  ```js
  // scan.js
  const aChecker = require("accessibility-checker");
  (async () => {
    const { report } = await aChecker.getCompliance("http://localhost:4200/ui/dc1/services", "services");
    for (const it of report.results) {
      if (it.level === "violation" || it.level === "potentialviolation") {
        console.log(`[${it.level}] ${it.ruleId} :: ${it.message}`);
        console.log("  path:", it.path && it.path.dom);
      }
    }
    await aChecker.close();
  })();
  ```
  Run with `node scan.js`. Swap the URL/label per page being audited.
- **Manual keyboard:** Tab/Shift+Tab/Enter/Escape/Arrow-key traversal, checking
  focus order, visible focus indication, keyboard operability of all
  interactive elements (dropdowns, modals, tables, forms), and no keyboard traps.
- **Screen-reader-equivalent spot checks:** inspection of the accessibility
  tree (roles, accessible names, states, live regions) that Chrome DevTools
  exposes. **This is not a live run of VoiceOver/NVDA/JAWS** — no screen
  reader automation is available in this environment. Tree data is a good
  proxy for what would be announced, but doesn't substitute for a real AT
  pass before shipping fixes.

Findings use axe's severity vocabulary: **Critical / Serious / Moderate / Minor**.

## Environment used

Dev server at `localhost:4200`, with `packages/consul-ui/config/environment.js`
`operatorConfig` set to `ACLsEnabled: true, NamespacesEnabled: true,
PeeringEnabled: true, PartitionsEnabled: true` (already the case on this
branch), `V2CatalogEnabled: false`, `HCPEnabled: false`. The V2 catalog / HCP
shell is a different application mode, not just extra routes, and is **out of
scope** for this pass.

Run with `pnpm start` (the mock API, see [mock-api](../../mock-api)) — export
`CONSUL_SERVICE_MIN=2` first so the randomly-generated service count can
never collapse to a single, non-mesh service (see
[task-4-mock-data-plan.md](task-4-mock-data-plan.md) for why that matters). A
stable `backend` service with a sidecar proxy — real `Upstreams` and
`Expose.Paths` — is now always present in the services list regardless of
that random count, so `dc.services.instance.upstreams`/`.exposedpaths` have
a reliable, reproducible target to audit against.

## Tasks

| # | Task | Routes | Status | File |
|---|------|--------|--------|------|
| 1 | Shared chrome & global patterns | app chrome, `settings`, `unavailable`, `notfound` | Complete (7 findings) | [01-shared-chrome.md](01-shared-chrome.md) |
| 2 | Overview | `dc.show`(`.serverstatus`/`.cataloghealth`/`.license`) | Complete (1 new finding + confirmed recurrences) | [02-overview.md](02-overview.md) |
| 3 | Services (list & detail) | `dc.services`(`.show`.*), `dc.routing-config` | Complete (3 new findings) | [03-services.md](03-services.md) |
| 4 | Service instance detail | `dc.services.instance`.* | Complete (1 finding + confirmed recurrences on all 5 sub-tabs) | [04-service-instance.md](04-service-instance.md) |
| 5 | Nodes | `dc.nodes`(`.show`.*) | Complete (1 high-reach finding) | [05-nodes.md](05-nodes.md) |
| 6 | Key/Value | `dc.kv`.* | Complete (2 minor findings) | [06-kv.md](06-kv.md) |
| 7 | Intentions (top-level) | `dc.intentions`.* | Complete (1 finding, upstream HDS) | [07-intentions.md](07-intentions.md) |
| 8 | Access Controls — Tokens | `dc.acls.tokens`.* | Complete (no new findings; 1 tool false-positive documented) | [08-acls-tokens.md](08-acls-tokens.md) |
| 9 | Access Controls — Policies | `dc.acls.policies`.* | Complete (no new findings) | [09-acls-policies.md](09-acls-policies.md) |
| 10 | Access Controls — Roles | `dc.acls.roles`.* | Complete (no new findings) | [10-acls-roles.md](10-acls-roles.md) |
| 11 | Access Controls — Auth Methods | `dc.acls.auth-methods`.* | Complete (no new findings; corroborates #3) | [11-acls-auth-methods.md](11-acls-auth-methods.md) |
| 12 | Peers | `dc.peers`.* | Complete (no new findings; `.show` blocked on no peered clusters in mock data) | [12-peers.md](12-peers.md) |
| 13 | Admin Partitions | `dc.partitions`.* | Complete (no new findings) | [13-partitions.md](13-partitions.md) |
| 14 | Namespaces | `dc.nspaces`.* | Complete (1 finding, 3rd occurrence of a recurring pattern) | [14-namespaces.md](14-namespaces.md) |

Task 1 (shared chrome) is audited first since its components (header, side
nav, tables, pagination, modals, notifications) are reused across every other
section — findings there apply everywhere and don't need to be re-logged
per-section.

## Summary — all 14 tasks

All tasks are complete except Task 12, which is partial: `dc.peers.show` has
no peered cluster to open in this dataset. (Task 4 was in the same state —
`dc.services.instance`'s `.upstreams`/`.exposedpaths`/`.addresses`/`.metadata`
tabs needed mesh mock data before they could be audited — but that's now
resolved; see [task-4-mock-data-plan.md](task-4-mock-data-plan.md) and
[04-service-instance.md](04-service-instance.md).) Everything else was fully
reachable.

**Recurring patterns worth fixing once, centrally, rather than per-instance:**

1. **`aria-label`/`aria-labelledby` on an element with no compatible role** (the
   attribute is silently dropped because the element's implicit role is
   `generic`, which doesn't support naming) — found independently 4 times:
   `<header>` wrappers ([02-overview.md](02-overview.md) #1), HDS's own
   `Hds::Alert` ([07-intentions.md](07-intentions.md) #1, recurring again on
   the service-instance Exposed Paths tab's intro alert —
   [04-service-instance.md](04-service-instance.md)), and the ACL
   ruleset badge ([14-namespaces.md](14-namespaces.md) #1, the one instance
   with a confirmed real information loss). Worth an `ember-template-lint`
   rule if one doesn't already exist for this.
2. **HDS's `{{hds-tooltip}}` modifier makes any host element tabbable with no
   widget role** ([01-shared-chrome.md](01-shared-chrome.md) #7) — recurred in
   nearly every task (Overview, Services, Service instance, Nodes, Peers).
   Upstream in `@hashicorp/design-system-components`; consul-ui uses it 30+
   places.
3. **`ConsulCopyButton`'s `aria-label` replaces its visible value instead of
   including it** ([05-nodes.md](05-nodes.md) #1) — WCAG 2.5.3 Label in Name,
   11 usage sites app-wide. Single highest-reach fix in the whole audit: one
   component change covers all 11.
4. **The side-nav `<aside>` has no accessible name** despite HDS rendering a
   hidden heading that was clearly meant to be it
   ([01-shared-chrome.md](01-shared-chrome.md) #5) — confirmed on every single
   page audited. One-line fix (`aria-label` on `Frame.Sidebar`).
5. **Side-nav/footer text contrast** (`#8c909c` on `#fafafa`, 3.05:1 vs the
   4.5:1 required) — [01-shared-chrome.md](01-shared-chrome.md) #1, confirmed
   on every single page audited, both by axe/Lighthouse and IBM's checker.

**The two most significant individual (non-recurring) findings:**

- The health-checks list loses all `<ul>`/`<li>` semantics because
  `display: flex` is applied to `<li>` for its layout, stripping the implicit
  `list`/`listitem` roles Chrome would otherwise assign — confirmed on both
  `dc.services.instance.healthchecks` and `dc.nodes.show.healthchecks`
  ([04-service-instance.md](04-service-instance.md) #1). ~20 checks read as
  one undifferentiated wall of text with no way to navigate check-by-check.
- The service topology diagram conveys health status by color alone (bare
  "45%9%45%" with no passing/warning/critical label) and has no structural/
  textual equivalent of the visual graph at all
  ([03-services.md](03-services.md) #1–#2). Neither automated engine caught
  either of these — both came from reading the topology component's source
  directly, a reminder that automated tools have real blind spots around
  custom visualizations.

**On tooling:** two IBM `accessibility-checker` results were investigated and
turned out to be false positives rather than real bugs — a `combobox_haspopup_valid`
complaint about Ember Power Select's portal wrapper (the real listbox
underneath is correctly implemented — [08-acls-tokens.md](08-acls-tokens.md)),
and the CSS-driven focus-ring pattern initially looked broken via
`getComputedStyle` until the `::before`-pseudo-element ring was checked
directly ([01-shared-chrome.md](01-shared-chrome.md)). Both are recorded in
their task files so they aren't rediscovered and mis-reported as bugs later.
