# Tags (`TagList`) — HDS migration

Migrate the shared `TagList` component from the legacy `<dl>` / CSV-list layout
to the HDS **Card + Tag pills** design shown in the Figma
(`HDS-Migration`, node `3376-125619`).

`TagList` is shared, so this single change updates **both** consumers:

- **Service "Tags" tab** — `/ui/{dc}/services/{name}/tags`
  (`app/templates/dc/services/show/tags.hbs`).
- **Service-instance "Tags" section** — the metadata tab
  (`app/templates/dc/services/instance/metadata.hbs`, under its `<h2>Tags</h2>`).

## Goal / before → after

**Before:** a `<dl class="tag-list">` with a `<dt>` "Tags" label (tag-icon mask +
tooltip) and a `<dd>` rendering each tag as a bare `<span>` in a comma-separated
(`%csv-list`) run.

**After:** an `Hds::Card::Container` (white surface, `24px` padding, `6px`
radius, subtle border + low elevation) containing a `flex-wrap` row of
`Hds::Tag` pills. No inline "Tags" label — the tab/section heading already names
it, matching the Figma.

### Layout (from Figma node `3376-125670`)

| Property      | Value                                                   |
| ------------- | ------------------------------------------------------- |
| Card chrome   | `Hds::Card::Container @level="mid" @hasBorder`           |
| Card padding  | `24px`                                                   |
| Tags row      | `display: flex; flex-wrap: wrap;` — `gap: 12px 8px`      |
| Pill          | `Hds::Tag @text={{tag}}` (default `primary` color)       |

The Figma maps each pill to the Helios **Tag** component, and `Hds::Tag` is
already used for the tag pills in the Service / Service-instance tables — so the
pill styling (border, `20px` radius, `13px` medium text, `foreground-primary`)
comes from HDS directly with no bespoke CSS.

## File-by-file changes

### 1. `app/components/tag-list/index.hbs`

Non-block branch replaced: `<dl>` → `Hds::Card::Container` wrapping
`<div class="tag-list__tags" data-test-tags>` with one `<Hds::Tag @text />` per
tag. `data-test-tags` moves onto the tags container (the direct parent of the
pills). The recursive `has-block` yield branch is unchanged.

### 2. `app/components/tag-list/index.scss`

Rewritten: dropped the `%tag-list` / `%horizontal-kv-list` / `%csv-list`
`@extend`s and the unused `td.tags` selector. Now just card padding + the
flex-wrap tags row with the Figma gaps.

### 3. Tab templates — unchanged

`services/show/tags.hbs` and `services/instance/metadata.hbs` keep their `Route`
wrappers, empty states, and (for the instance) the `<h2>Tags</h2>` heading.

## Tests / page objects

`Hds::Tag` renders each pill as `<span class="hds-tag">…</span>`, so it remains
the first-child span under `[data-test-tags]` in order — the existing feature
assertions (`[data-test-tags] span:nth-child(n)` → "Tag1" / "Tag2" / "Tag3") in
`services/show.feature` and `services/instances/show.feature` continue to pass
via the `find()`-based `see the text … in …` step.

The one page-object collection that targeted the old `<dd> > span` markup was
updated:

- `tests/pages/dc/services/show.js` — `.tag-list dd > span` → `.tag-list .hds-tag`.

## Testing

- `ember test --filter "Show Service"` — 15 pass (includes the tags-display
  scenario and the instance tags-section assertions).
- Visual QA at `/ui/dc1/services/{name}/tags` and
  `/ui/dc1/services/{name}/instances/{id}/metadata`: tags render as pills inside
  the card, wrapping across rows; empty states unchanged.

## Out of scope

- No routing / data changes — `TagList`'s `@item` / `@tags` API is unchanged.
- The recursive block-form (`has-block`) usage is preserved as-is.
