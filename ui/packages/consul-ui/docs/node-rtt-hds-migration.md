# Node Detail — Round Trip Time Tab — HDS Migration

**Branch:** `suresh/nodes-hds-migration`
**Page:** `/ui/dc1/nodes/node-0/round-trip-time` (route `dc.nodes.show.rtt`)
**Figma:** [HDS-Migration — node `3324:70738`](https://www.figma.com/design/Hpmmh25EIyt0ZM0Jsv8xT9/HDS-Migration?node-id=3324-70738&m=dev)
**Type:** HDS Migration
**Scope:** The Round Trip Time tab body on a single node's detail page only — the min/median/max
stats block and the radial tomography graph. Follow-on to
[`nodes-hds-migration.md`](./nodes-hds-migration.md), which deferred "Node detail page & tabs" to
separate tickets. Sibling of [`node-healthchecks-hds-migration.md`](./node-healthchecks-hds-migration.md)
and [`node-lock-sessions-hds-migration.md`](./node-lock-sessions-hds-migration.md).

---

## Overview

The Figma target redesigns the Round Trip Time tab so its whole body sits inside a single **HDS
card**, with the min/median/max stats and the radial graph laid out **side by side** (separated by a
vertical divider) instead of stacked. Four things change, all presentational:

1. The tab body is wrapped in an **`Hds::Card::Container`** (white surface, 1px border, low
   elevation shadow, `6px` radius, `24px` padding) — currently the content sits bare in a
   `.tab-section` with no card chrome.
2. The stats and the graph move from **stacked** (stats above graph) to a **horizontal two-column**
   layout with a **vertical separator** between them (Figma: `flex`, `gap: 64px`, `items: start`).
3. The legacy `.definition-table` `<dl>/<dt>/<dd>` stats block is replaced with a simple two-column
   label/value layout using HDS body typography, and the labels gain a "time" suffix —
   **"Minimum time" / "Median time" / "Maximum time"** (currently "Minimum" / "Median" /
   "Maximum").
4. The `Consul::Tomography::Graph` SVG itself is **unchanged** — its colors are already tokenized.
   Only its container/placement changes.

No backend, data-model, or route-redirect changes are required — the `rtt.js` redirect (bounce to
`dc.nodes.show` when `tomography.distances` is empty) stays as-is.

---

## Files in Scope

| File | Change type | Notes |
|---|---|---|
| `app/templates/dc/nodes/show/rtt.hbs` | HBS | Wrap body in `Hds::Card::Container`; two-column layout; add vertical separator; rename stat labels; drop `.definition-table` `<dl>` |
| `app/styles/routes/dc/nodes/index.scss` | CSS | New scoped rules for the RTT card's two-column layout + stats typography (scoped to `dc.nodes.show.rtt`) |
| `app/components/consul/tomography/graph/index.scss` | CSS | `.background` disc fill `surface-strong` → `transparent` to match the Figma white field (see §4). Only usage of this component is the RTT tab. |
| `app/routes/dc/nodes/show/rtt.js` | No change | Empty-distances redirect unchanged |
| `translations/**` | No change | Labels kept as inline literals, consistent with the pre-migration template (see §3) |

---

## Comparison: Current vs Target

### 1. Card chrome — bare `.tab-section` → `Hds::Card::Container`

| | Current | Target |
|---|---|---|
| **Visual** | Stats `<dl>` and graph sit directly in `.tab-section` with no border/background. | Whole body in a white card: `--token-color-surface-primary` bg, 1px border, **Elevation/Surface/Low** shadow, `border-radius: 6px`, `24px` padding, overflow clipped. |
| **Figma** | — | Card node `3324:70797`: `bg surface-primary`, `p-24`, `rounded 6px`, `shadow 0 0 0 1px rgba(101,106,118,.15), 0 1px 1px rgba(...,.05), 0 2px 2px rgba(...,.05)`, `overflow-clip`. |
| **Implementation** | — | Reuse the established card primitive: `<Hds::Card::Container @level="mid" @hasBorder={{true}} @overflow="hidden">` — same invocation as `consul/server/card` and `consul/peer/bento-box`. `@level="mid"` maps to the Elevation/Surface/Low shadow in the Figma. |
| **Status** | ⚠️ **Change required.** | — |

> Note: unlike the health-checks / service-instances **toolbar** cards (which used
> `--token-color-surface-faint` + `--token-color-border-faint`), this content card uses the standard
> `Hds::Card::Container` white surface + elevation shadow, matching the Figma exactly.

### 2. Layout — stacked → side-by-side with vertical separator

| | Current | Target |
|---|---|---|
| **Visual** | Stats block on top, graph below (vertical stack in `.tab-section`). | Stats column on the **left**, graph on the **right**, side by side, with a **vertical divider** between them. Figma: `flex`, `gap: 64px`, `align-items: start`; separator is `w-px h-[329px]`. |
| **Implementation** | — | Card body is a horizontal flex container (`gap: 64px`, `align-items: flex-start`). Left: the stats column. Middle: a `1px` bordered `<div class="consul-node-rtt__divider">` that `align-self: stretch`es to the graph height. Right: `<Consul::Tomography::Graph>`. |
| **Status** | ✅ **Done.** `Hds::Separator` was rejected — it renders a horizontal `<hr>` with only a `@spacing` arg (no orientation), so the bordered-`<div>` divider is cleaner. |

### 3. Stats block — `.definition-table` `<dl>` → two-column label/value + "time" labels

| | Current | Target |
|---|---|---|
| **Visual** | `<dl><dt>Minimum</dt><dd>0.12ms</dd>…` styled by legacy `%definition-table` (dt = `display-100-semibold`, small). | Three rows, each `[label]  [value]`. Label = **Body/300/Semibold** (16px / 600 / lineHeight 24, color `#0c0c0e` → `--token-color-foreground-strong`), fixed `120px` width. Value = **Body/300/Regular** (16px / 400). Row gap `20px`, label→value gap `16px`. Labels read **"Minimum time" / "Median time" / "Maximum time"**. |
| **Current code** | `rtt.hbs` lines 14–35 — `<div class="definition-table"><dl>…</dl></div>`. | Replace with a small flex-column block of three `[label][value]` rows. Keep the `{{format-number … maximumFractionDigits=2}}ms` value formatting exactly. |
| **Status** | ⚠️ **Change required** — markup + typography + label copy. | — |

**Copy decision (resolved):** labels are kept as **inline literals** ("Minimum time" etc.),
consistent with the pre-migration template. No new i18n keys were added.

### 4. Tomography graph — one background fix

| | Current | Target |
|---|---|---|
| **Visual** | Radial SVG with a **grey filled disc** (`--token-color-surface-strong`), `neutral-300` dashed axis rings + solid border, `consul-foreground` (pink) spokes + center point, `neutral-300`/`foreground-strong` tick labels. | Radial graph on the card's **white field** — only dashed ring outlines, no grey disc; pink spokes; ms tick labels to the right. |
| **Code** | `consul/tomography/graph/index.scss` `.background { fill: var(--token-color-surface-strong); }`. | `.background { fill: transparent; }` — the only change to the component. SVG geometry, sampling, spokes, ticks, and the `336px` size are unchanged. This component is used **only** on the RTT tab, so the change is contained. |
| **Status** | ✅ **Done** — single-line fill change (small deviation from the original "no change" scope, to match the Figma white field). |

> Ring geometry differs slightly (current draws dashed rings at 25/50/75/100%; the Figma shows
> 33/66/100%). This was **left as-is** — changing it would also move the tick-label math, a deeper
> change than this presentational pass warrants.

---

## Task Checklist

- [x] Wrap the `rtt.hbs` body in `<Hds::Card::Container @level="mid" @hasBorder={{true}} @overflow="hidden">`
- [x] Make the card body a horizontal flex (`gap: 64px`, `align-items: start`) — stats left, graph right
- [x] Replace the `.definition-table` `<dl>` with a flex-column of three `[label 120px semibold][value regular]` rows, gap `20px`
- [x] Rename labels to "Minimum time" / "Median time" / "Maximum time" (kept as inline literals — see §3)
- [x] Preserve `{{format-number … maximumFractionDigits=2}}ms` value formatting
- [x] Add a vertical divider between stats and graph (`1px` bordered `<div>`; `Hds::Separator` is horizontal-only)
- [x] Fix the graph `.background` disc fill to `transparent` to match the Figma white field (§4)
- [x] Add scoped SCSS in `styles/routes/dc/nodes/index.scss` (`html[data-route^='dc.nodes.show.rtt']`) for layout + stats typography; global `%definition-table` untouched
- [x] Keep the `{{#if (gt tomography.distances.length 0)}}` guard and the `rtt.js` empty-distances redirect
- [ ] Run `ember test --filter "nodes/show"` — confirm no regressions
- [x] Visual QA at `http://localhost:4200/ui/dc1/nodes/node-1/round-trip-time` against the Figma

---

## Acceptance Criteria

1. The RTT tab body renders inside a single white `Hds::Card::Container` with a 1px border, low
   elevation shadow, `6px` radius, and `24px` interior padding.
2. Stats and graph are laid out side by side, stats on the left, graph on the right, with a vertical
   separator between them.
3. The stats show three rows — **Minimum time / Median time / Maximum time** — each with a `120px`
   semibold label and a regular value, using Body/300 (16px) typography.
4. Values keep the current formatting (`format-number` to 2 fraction digits + `ms`).
5. The radial tomography graph is visually unchanged from today.
6. When a node has no tomography distances, the tab still redirects to `dc.nodes.show` (unchanged).
7. No visual regressions on other node tabs or on any other page that uses `%definition-table`
   (the new styles must be scoped to `dc.nodes.show.rtt`).

---

## Testing Notes

- Run `ember test --filter "nodes/show"` to catch template/selector regressions on the node detail
  route.
- Visually verify at `http://localhost:4200/ui/dc1/nodes/node-1/round-trip-time` against the Figma
  frame (`3324:70738`), checking card chrome, the two-column layout, the vertical separator, and the
  "…​ time" labels.
- Confirm `%definition-table` styling elsewhere (e.g. intentions edit) is unaffected — the new rules
  must be route-scoped, not global.
- Confirm a node with no RTT data still bounces back to the node overview tab.

---

## Out of Scope (this ticket)

- `Consul::Tomography::Graph` internals (SVG geometry, sampling, colors) — reused unchanged.
- Node header (name / status badge / "Imported from billing" badge / address) and breadcrumbs —
  handled elsewhere.
- Tab navigation bar — already migrated.
- Backend / API / model / `rtt.js` redirect logic — no changes.
