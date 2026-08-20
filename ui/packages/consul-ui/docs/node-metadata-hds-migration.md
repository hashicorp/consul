# Node "Metadata" tab — HDS migration

Migrate the node detail **Metadata** tab (`/ui/{dc}/nodes/{name}/metadata`) from
the legacy `TabularCollection` table to the HDS **Card** layout shown in the
Figma design (`HDS-Migration`, node `3324-78030`).

Because the same `Consul::Metadata::List` component also renders the
service-instance metadata tab (`/ui/{dc}/services/{name}/instances/{id}/metadata`),
this single change migrates **both** tabs consistently.

## Goal / before → after

**Before:** `Consul::Metadata::List` rendered a `TabularCollection` with `Key` /
`Value` `<th>`/`<td>` columns and row striping — a full data-grid treatment.

**After:** a single `Hds::Card::Container` (white surface, `24px` padding,
`6px` radius, subtle border + low elevation) containing a two-column
**Key / Value** layout. No row dividers, no sortable headers, no toolbar, no
search, no copy button — a read-only description card, per Figma.

### Layout (from Figma node `3324-78089`)

| Property        | Value                                             |
| --------------- | ------------------------------------------------- |
| Card chrome     | `Hds::Card::Container @level="mid" @hasBorder`     |
| Card padding    | `24px`                                             |
| Columns         | Key (fixed `300px`) + Value (fills remaining)      |
| Column gap      | `32px`                                             |
| Row gap         | `16px`                                             |
| Header weight   | semibold (`Key` / `Value`)                         |
| Body weight     | regular                                            |
| Typography      | `body-300` (16px / 24px), `foreground-strong`      |
| Wrapping        | long keys/values wrap (`word-break: break-word`)   |

The consul logo shown before `v1.17.2` in the mock is **sample data**, not a
generic feature — metadata values are arbitrary strings and are rendered as-is.

## Architecture — CSS grid, not two stacks

The card uses a CSS **grid** (`grid-template-columns: 300px 1fr`) rather than two
independent flex columns so that Key and Value rows stay aligned even when a key
wraps onto multiple lines (e.g. the long `consul-dashboard-url…` sample). The
header cells (`Key` / `Value`) and each `dt` / `dd` pair are direct grid items.

## File-by-file changes

### 1. `app/components/consul/metadata/list/index.hbs`

Replace `TabularCollection` with `Hds::Card::Container` wrapping a `<dl>` grid:

- `data-test-metadata` moves onto the card container (preserved).
- Each entry renders `<dt data-test-metadata-key>` + `<dd data-test-metadata-value>`.

### 2. `app/components/consul/metadata/list/index.scss` (new)

Grid + typography tokens (`--token-typography-body-300-*`,
`--token-color-foreground-strong`, `--token-typography-font-weight-*`). Mirrors
the tokens used by the RTT card migration.

### 3. `app/styles/components.scss`

Register the new stylesheet:
`@import 'consul-ui/components/consul/metadata/list';`

### 4. `app/components/consul/metadata/list/index.js`

Unchanged — classic `tagName: ''` component; public `@items` API is unchanged
(`[key, value]` pairs from `entries`).

### 5. Tab templates — unchanged

`app/templates/dc/nodes/show/metadata.hbs` and
`app/templates/dc/services/instance/metadata.hbs` keep their `Route` wrapper and
`EmptyState` blocks; only the shared component's internals changed.

## Tests / page objects

The old markup emitted `[data-test-tabular-row]` per row (from
`TabularCollection`); the new grid emits one `[data-test-metadata-key]` per row.
Both metadata collections were updated to count that selector instead:

- `tests/pages/dc/nodes/show.js` — `.consul-metadata-list [data-test-metadata-key]`
- `tests/pages/dc/services/instance.js` — `.metadata [data-test-metadata-key]`

The `show.feature` metadata-count scenarios (e.g. "I see 3 of the metadata
object") continue to pass against the new selector.

## Testing

- `ember test --filter metadata`
- Visual QA at `/ui/dc1/nodes/{node}/metadata` and
  `/ui/dc1/services/{name}/instances/{id}/metadata`: card renders, Key/Value rows
  align, long keys wrap, and the empty state ("This node/instance has no
  metadata.") is unchanged when `Meta` is absent.

## Out of scope

- Toolbar / search / sort / pagination — not present in the Figma for this tab.
- Copy buttons on values — not in the Figma.
- Route query params / controller — none needed (no search/sort/filter state).
