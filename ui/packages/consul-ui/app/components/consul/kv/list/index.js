/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { htmlSafe } from '@ember/template';
import keyName from 'consul-ui/utils/keyName';

// KV is a tree, so sorting stays in the toolbar rather than the column headers.
const COLUMNS = [
  { label: 'Key name', width: '100%' },
  { label: 'Actions', align: 'right', width: '80px' },
];

// Indentation (px) per level of nesting.
const INDENT_STEP = 20;

const PAGE_SIZES = [10, 30, 50, 100];

export default class ConsulKvList extends Component {
  @service('sort') sortService;

  columns = COLUMNS;
  pageSizes = PAGE_SIZES;

  // Keys of the folders that are currently expanded.
  @tracked expandedKeys = new Set();

  // Loaded children keyed by folder Key, populated by the per-folder
  // <DataSource> as data arrives.
  @tracked childrenByKey = {};

  // Holds the pending KV entry while its delete confirmation modal is open.
  @tracked itemToDelete = null;

  @tracked page = 1;
  @tracked pageSize = PAGE_SIZES[0];

  // Paging counts the entries of the folder being listed, never the children an
  // expanded folder pulls in: those render within their parent's page.
  get totalItems() {
    return (this.args.items || []).length;
  }

  // Clamped rather than reset, so filtering down to fewer pages while on a
  // later one lands on the last page instead of on nothing.
  get currentPage() {
    return Math.min(this.page, Math.max(1, Math.ceil(this.totalItems / this.pageSize)));
  }

  get pagedItems() {
    const start = (this.currentPage - 1) * this.pageSize;
    return (this.args.items || []).slice(start, start + this.pageSize);
  }

  // This page's entries plus the loaded children of any expanded folder,
  // flattened with their tree metadata.
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
          out.push(...build(this.sortChildren(this.childrenByKey[node.Key]), depth + 1));
        }
      });
      return out;
    };
    return build(this.pagedItems, 0);
  }

  // A folder's children arrive from their own request in the API's order, so
  // they are sorted the same way the toolbar sorts the entries above them.
  sortChildren(children = []) {
    // e.g. 'Kind:asc'; the comparator falls back to the first sortable property
    const [definition] = this.sortService.comparator('kv')(this.args.sort);
    const [property, direction] = definition.split(':');
    const order = direction === 'desc' ? -1 : 1;
    const compare = (a, b) => (a > b ? 1 : a < b ? -1 : 0);
    return [...children].sort((a, b) => {
      // entries of the same Kind stay alphabetical, as they are at the level above
      return compare(a[property], b[property]) * order || compare(a.Key, b.Key);
    });
  }

  // The folder keys whose children need a live <DataSource> subscription.
  get expandedFolders() {
    return Array.from(this.expandedKeys);
  }

  get itemToDeleteName() {
    return this.displayName(this.itemToDelete?.Key);
  }

  displayName(key) {
    return keyName(key) || key;
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
  onPageChange(page) {
    this.page = page;
  }

  @action
  onPageSizeChange(size) {
    this.pageSize = size;
    this.page = 1;
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
