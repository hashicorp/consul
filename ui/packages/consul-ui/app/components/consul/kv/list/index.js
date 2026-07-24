/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { htmlSafe } from '@ember/template';

// Column definitions for the KV table. KV is a hierarchical tree, so it is not
// client-sortable at the column level (sorting stays in the toolbar) and it is
// rendered without pagination (paging would split a parent folder from its
// children). Cell rendering lives in the template's :row block.
const COLUMNS = [
  { label: 'Key name', width: '100%' },
  { label: 'Actions', align: 'right', width: '80px' },
];

// Horizontal indentation (px) applied per level of nesting in the tree.
const INDENT_STEP = 20;

/**
 * Consul::Kv::List
 *
 * KV specific configuration for the generic Consul::DataTable, rendered as an
 * inline expand/collapse tree. The top-level rows are the already
 * fetched/filtered/searched `@items` (the current folder's immediate children);
 * expanding a folder lazily loads *its* children via a `<DataSource>` and
 * splices them into the visible rows at the next depth. Navigation into a
 * folder and delete are preserved through the per-row actions menu.
 */
export default class ConsulKvList extends Component {
  columns = COLUMNS;

  // Keys of the folders that are currently expanded.
  @tracked expandedKeys = new Set();

  // Loaded children keyed by folder Key: { [folderKey]: Kv[] }. Populated by
  // the per-folder <DataSource> as data arrives / live-updates.
  @tracked childrenByKey = {};

  // Holds the pending KV entry while its delete confirmation modal is open.
  @tracked itemToDelete = null;

  // The flattened list of currently-visible rows (roots plus the loaded
  // children of any expanded folder), each wrapped with its tree metadata.
  get rows() {
    const build = (nodes, depth) => {
      const out = [];
      (nodes || []).forEach((node) => {
        const expanded = node.isFolder && this.expandedKeys.has(node.Key);
        out.push({
          node,
          depth,
          isFolder: node.isFolder,
          expanded,
          name: this.displayName(node.Key),
          indentStyle: htmlSafe(`padding-inline-start:${depth * INDENT_STEP}px`),
        });
        if (expanded) {
          out.push(...build(this.childrenByKey[node.Key], depth + 1));
        }
      });
      return out;
    };
    return build(this.args.items, 0);
  }

  // The folder keys whose children need a live <DataSource> subscription.
  get expandedFolders() {
    return Array.from(this.expandedKeys);
  }

  // The last non-empty path segment of a Key, e.g. "a/b/c" -> "c" and the
  // folder "a/b/" -> "b". Mirrors the legacy display, which trimmed the parent
  // prefix and any trailing slash.
  displayName(key) {
    const parts = (key || '').split('/').filter(Boolean);
    return parts[parts.length - 1] || key;
  }

  @action
  toggle(node) {
    const next = new Set(this.expandedKeys);
    if (next.has(node.Key)) {
      next.delete(node.Key);
    } else {
      next.add(node.Key);
    }
    this.expandedKeys = next;
  }

  @action
  setChildren(key, event) {
    this.childrenByKey = { ...this.childrenByKey, [key]: event.data };
  }

  @action
  confirmDelete(item) {
    this.itemToDelete = item;
  }

  @action
  cancelDelete() {
    this.itemToDelete = null;
  }

  @action
  invokeDelete() {
    const item = this.itemToDelete;
    this.itemToDelete = null;
    if (item) {
      this.args.delete(item);
    }
  }
}
