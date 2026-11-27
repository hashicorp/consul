# Scenario: kv

Seeds a small, deterministic Key/Value tree to exercise the **Key/Value**
browser (`Consul::Kv::Browser` / `Consul::Kv::List`) — standalone top-level
keys, folders nested 1 to 4 levels deep, a single chain 15 levels deep,
names long enough to overflow the tree column, and one folder that's
explicitly empty.

## Tree

### Everyday shapes (1-4 levels)

```
readme                                     "Sample KV data seeded by the kv scenario for the Key/Value browser UI."
maintenance-mode                           "false"
build-number                               "42"

config/
config/timeout                             "30s"
config/retries                             "3"

app/
app/settings/
app/settings/theme                         "dark"
app/settings/locale                        "en-US"
app/services/
app/services/billing/
app/services/billing/owner                 "team-payments"
app/services/billing/flags/
app/services/billing/flags/dark-launch     "true"

archive/
archive/2023/                              (empty folder — no keys underneath)
```

### Deep nesting (15 levels)

`deep/` is level 1 and `level-02/`..`level-15/` the rest. Only levels 1, 3, 7
and 12 carry a `depth` key next to their child folder, so the chain has both
**mixed** levels (a key and a folder side by side) and **folder-only** levels:

```
deep/                                              level 1
deep/depth                                 "1"
deep/level-02/                                     level 2   (folder only)
deep/level-02/level-03/                            level 3
deep/level-02/level-03/depth               "3"
...
deep/.../level-07/depth                    "7"     level 7
...
deep/.../level-12/depth                    "12"    level 12
...
deep/.../level-15/                                 level 15  (deepest folder)
deep/.../level-15/deepest-key              "Fifteen folders deep — the most indented row in the tree."
deep/.../level-15/a-key-with-a-…-table-row "Deepest indentation and an overflowing name at once."
```

The full deepest path is 218 characters — Consul caps a key at 512, so this
stays well inside it.

### Long names

```
a-folder-with-a-deliberately-long-name-…-tree-column/
  short-key                                "A short key inside a folder with a very long name."
  a-key-with-a-deliberately-long-name-…-table-row
                                           "Both the folder and the key have overflowing names."
  another-very-long-folder-name-…-overflow/
    nested-key                             "Two long-named folders deep."
```

## Notes

- `readme`, `maintenance-mode`, `build-number` — standalone top-level keys, no folder.
- `config/` — a 1-level folder with two keys.
- `app/settings/` — a 2-level folder (`app/` → `app/settings/`) with two keys.
- `app/services/billing/` — a 3-level folder with a key (`owner`) directly inside it.
- `app/services/billing/flags/` — a 4-level folder with a key (`dark-launch`).
- `deep/` — a 15-level chain. The list indents by 20px per level
  (`INDENT_STEP` in `app/components/consul/kv/list/index.js`), so its deepest
  row sits ~280px into the column; combined with the long-named key at that
  level it's the worst case for the "Key name" column.
- `archive/2023/` — an **explicitly empty** 2-level folder: it's the only
  entry written with a `/`-suffixed key and empty value (Consul reads it
  back as `Value: null`), since Consul/consul-ui otherwise derive folders
  purely from `/`-separated prefixes of real keys — a folder with no keys
  under it needs its own marker to show up at all.

19 entries in total; the root level lists 8 of them (3 keys + 5 folders),
which is under the list's default page size of 10.

Consul's KV write API takes the raw value as the PUT body (not JSON), so
`seed.ts` uses `ConsulClient#putKV(key, value)` (added alongside a `kv(prefix?)`
read helper that base64-decodes `Value`) rather than the JSON-only
`request()`/`json()` helpers used everywhere else in the client.

- UI: `http://localhost:4200/ui/dc1/kv`
- Drill into a folder directly, e.g.
  `http://localhost:4200/ui/dc1/kv/app/services/billing/flags/` or
  `http://localhost:4200/ui/dc1/kv/deep/`
- Tests: [`tests/kv/`](../../tests/kv/).
