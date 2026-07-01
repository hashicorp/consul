/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

const DEFAULT_PAGE_SIZES = [10, 30, 50, 100];

/**
 * Consul::DataTable
 *
 * A generic, presentational table built on the HashiCorp Design System
 * `Hds::Table`. It owns nothing but display concerns: client-side column
 * sorting and client-side pagination over the in-memory `@items` array it is
 * handed. It performs no data fetching, filtering or searching of its own, so
 * it can sit underneath any list page's existing data layer (DataLoader ->
 * DataCollection) without changing how data is loaded.
 *
 * Columns are described declaratively via `@columns`, and each row's cells are
 * supplied by the caller through the `:row` named block (which yields the row
 * `item` and the HDS body API `B`), keeping cell rendering — links, icons,
 * tooltips, etc. — entirely up to the consuming page.
 *
 * @argument {Array} items - the rows to display.
 * @argument {Array} columns - column definitions. Each entry supports:
 *   - `label` {string} the header text.
 *   - `sortKey` {string} [optional] makes the column sortable; identifies it.
 *   - `sortValue` {(item) => comparable} [optional] custom comparator value;
 *      defaults to a case-insensitive read of `item[sortKey]`.
 *   - `align` {'left'|'center'|'right'} [optional] header/column alignment.
 * @argument {Array} [pageSizes] - page-size options; defaults to [10,30,50,100].
 * @argument {string} [initialSortBy] - sortKey to sort by initially.
 * @argument {'asc'|'desc'} [initialSortOrder] - initial sort direction.
 * @argument {string} [density] - HDS Table density; defaults to "medium".
 * @argument {string} [valign] - HDS Table vertical alignment; defaults "middle".
 * @argument {string} [ariaLabel] - accessible label for the table.
 * @argument {number} [linkColumn] - 1-based index of the column that holds the
 *   clickable link; drives the pointer cursor and link hover styling. Defaults
 *   to 1 (the first column).
 */
export default class ConsulDataTable extends Component {
  @tracked page = 1;
  @tracked pageSize;
  @tracked sortBy;
  @tracked sortOrder = 'asc';

  // Whether the horizontally scrollable area has hidden content to the left or
  // right of what is currently visible. Drives the edge fade indicators so the
  // user can tell there are more columns to scroll to.
  @tracked canScrollLeft = false;
  @tracked canScrollRight = false;

  // The scrollable element, captured on insert so we can re-measure on resize.
  scrollElement = null;

  constructor() {
    super(...arguments);
    this.pageSize = this.pageSizes[0];
    this.sortBy = this.args.initialSortBy;
    if (this.args.initialSortOrder) {
      this.sortOrder = this.args.initialSortOrder;
    }
  }

  get pageSizes() {
    return this.args.pageSizes || DEFAULT_PAGE_SIZES;
  }

  // 1-based index of the column whose cell is clickable. Clamped to at least 1
  // so the styling hook always points at a real column.
  get linkColumn() {
    const value = parseInt(this.args.linkColumn, 10);
    return Number.isInteger(value) && value > 0 ? value : 1;
  }

  get columns() {
    return this.args.columns || [];
  }

  get items() {
    return this.args.items || [];
  }

  // Builds a map of sortKey -> comparator function from the column definitions.
  // A column with a `sortKey` but no `sortValue` falls back to a
  // case-insensitive read of `item[sortKey]`.
  get sortValueFns() {
    const map = {};
    this.columns.forEach((column) => {
      const key = column.sortKey;
      if (!key) {
        return;
      }
      map[key] =
        column.sortValue ||
        ((item) => {
          const value = item[key];
          return typeof value === 'string' ? value.toLowerCase() : value;
        });
    });
    return map;
  }

  get totalItems() {
    return this.items.length;
  }

  get sortedItems() {
    const items = this.items;
    const valueFor = this.sortValueFns[this.sortBy];
    if (!valueFor) {
      return items;
    }
    const direction = this.sortOrder === 'desc' ? -1 : 1;
    return [...items].sort((a, b) => {
      const av = valueFor(a);
      const bv = valueFor(b);
      if (av < bv) return -1 * direction;
      if (av > bv) return 1 * direction;
      return 0;
    });
  }

  get paginatedItems() {
    const start = (this.page - 1) * this.pageSize;
    return this.sortedItems.slice(start, start + this.pageSize);
  }

  @action
  setSortBy(key) {
    if (this.sortBy === key) {
      this.sortOrder = this.sortOrder === 'asc' ? 'desc' : 'asc';
    } else {
      this.sortBy = key;
      this.sortOrder = 'asc';
    }
    this.page = 1;
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

  // Captures the scrollable element, wires up a ResizeObserver so the edge
  // indicators stay correct when the layout changes, and takes an initial
  // measurement.
  @action
  setupScrollIndicators(element) {
    this.scrollElement = element;
    // Measure after the table has actually laid out; at insert time the HDS
    // table may not have its final width yet.
    requestAnimationFrame(() => this.updateScrollIndicators());
    if (typeof ResizeObserver !== 'undefined') {
      this.resizeObserver = new ResizeObserver(() => this.updateScrollIndicators());
      // Observe the scroll container (viewport width changes) AND its content
      // (the HDS table). Content overflow does not change the container's own
      // box, so observing only the container would never detect that the
      // columns are wider than the viewport.
      this.resizeObserver.observe(element);
      if (element.firstElementChild) {
        this.resizeObserver.observe(element.firstElementChild);
      }
    }
  }

  @action
  teardownScrollIndicators() {
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
      this.resizeObserver = null;
    }
    this.scrollElement = null;
  }

  // Recomputes whether there is hidden content to the left/right of the
  // viewport. A 1px tolerance avoids sub-pixel rounding leaving an indicator on
  // when the content is effectively flush with an edge.
  @action
  updateScrollIndicators() {
    const element = this.scrollElement;
    if (!element) {
      return;
    }
    const { scrollLeft, scrollWidth, clientWidth } = element;
    const maxScroll = scrollWidth - clientWidth;
    this.canScrollLeft = scrollLeft > 1;
    this.canScrollRight = scrollLeft < maxScroll - 1;
  }
}
