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
 * Sorting defaults to "uncontrolled": the table owns its own sort state
 * (seeded from `@initialSortBy`/`@initialSortOrder`) and re-sorts `@items`
 * itself whenever a sortable header is clicked. This is the right fit for
 * pages with no other sort control.
 *
 * Some pages also expose their own sort control (e.g. a toolbar dropdown) that
 * drives the data layer (DataCollection) to sort `@items` before they ever
 * reach this table. If this table *also* ran its own uncontrolled sort on top
 * of that, the two would fight over the displayed order (whichever one last
 * changed "wins", and the other silently appears to do nothing). Passing
 * `@sortBy`/`@sortOrder`/`@onSort` puts the table in "controlled" mode: it
 * reflects the caller's sort state instead of tracking its own, calls
 * `@onSort` instead of re-sorting locally when a header is clicked, and trusts
 * `@items` to already be in the right order — making the caller's sort state
 * the single source of truth for both controls.
 *
 * @argument {Array} items - the rows to display.
 * @argument {Array} columns - column definitions. Each entry supports:
 *   - `label` {string} the header text.
 *   - `sortKey` {string} [optional] makes the column sortable; identifies it.
 *   - `sortValue` {(item) => comparable} [optional] custom comparator value;
 *      defaults to a case-insensitive read of `item[sortKey]`. Ignored in
 *      controlled mode, since the table doesn't sort `@items` itself.
 *   - `align` {'left'|'center'|'right'} [optional] header/column alignment.
 * @argument {Array} [pageSizes] - page-size options; defaults to [10,30,50,100].
 * @argument {string} [initialSortBy] - sortKey to sort by initially. Ignored
 *   in controlled mode.
 * @argument {'asc'|'desc'} [initialSortOrder] - initial sort direction.
 *   Ignored in controlled mode.
 * @argument {string} [sortBy] - controlled current sortKey; puts the table in
 *   controlled mode together with `@onSort`.
 * @argument {'asc'|'desc'} [sortOrder] - controlled current sort direction.
 * @argument {(sortBy: string, sortOrder: 'asc'|'desc') => void} [onSort] -
 *   called with the new sortKey/sortOrder when a sortable header is clicked;
 *   presence of this argument is what puts the table in controlled mode.
 * @argument {string} [density] - HDS Table density; defaults to "medium".
 * @argument {string} [valign] - HDS Table vertical alignment; defaults "middle".
 * @argument {string} [ariaLabel] - accessible label for the table.
 * @argument {number} [linkColumn] - 1-based index of the column that holds the
 *   clickable link; drives the pointer cursor and link hover styling. Defaults
 *   to 1 (the first column).
 */
export default class ConsulDataTable extends Component {
  // Backing state for `page`; kept separate from the `page` getter below so
  // filtering/searching/deleting (which change `@items` but not this
  // directly) can't leave it pointing past the end of the new result set.
  @tracked _page = 1;
  @tracked pageSize;

  // Backing state for uncontrolled mode only; ignored (and left unset) in
  // controlled mode, where `sortBy`/`sortOrder` are read from `@args` instead.
  @tracked _sortBy;
  @tracked _sortOrder = 'asc';

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
    this._sortBy = this.args.initialSortBy;
    if (this.args.initialSortOrder) {
      this._sortOrder = this.args.initialSortOrder;
    }
  }

  // Controlled mode is opt-in: it's only active once the caller supplies
  // `@onSort`, so every other existing consumer of this table keeps its
  // current uncontrolled behaviour unchanged.
  get isControlled() {
    return typeof this.args.onSort === 'function';
  }

  get sortBy() {
    return this.isControlled ? this.args.sortBy : this._sortBy;
  }

  get sortOrder() {
    return (this.isControlled ? this.args.sortOrder : this._sortOrder) || 'asc';
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

  // Pagination is on by default. Callers that render a non-paginated view (for
  // example a hierarchical tree, where paging would split a parent from its
  // children) can opt out with `@paginated={{false}}`.
  get paginated() {
    return this.args.paginated !== false;
  }

  get totalPages() {
    if (!this.paginated || !this.pageSize) {
      return 1;
    }
    return Math.max(1, Math.ceil(this.totalItems / this.pageSize));
  }

  // Clamped to the current number of pages, so filtering, searching or
  // deleting @items down to a smaller (but still non-empty) result set can
  // never leave the table stuck on a now-out-of-range page showing no rows.
  get page() {
    return Math.min(this._page, this.totalPages);
  }

  get sortedItems() {
    const items = this.items;
    // In controlled mode the caller's data layer is the single source of
    // truth for ordering (see class docs); re-sorting here would fight it.
    if (this.isControlled) {
      return items;
    }
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
    if (!this.paginated) {
      return this.sortedItems;
    }
    const start = (this.page - 1) * this.pageSize;
    return this.sortedItems.slice(start, start + this.pageSize);
  }

  @action
  setSortBy(key) {
    if (this.isControlled) {
      const nextOrder = this.sortBy === key && this.sortOrder === 'asc' ? 'desc' : 'asc';
      this.args.onSort(key, nextOrder);
    } else if (this._sortBy === key) {
      this._sortOrder = this._sortOrder === 'asc' ? 'desc' : 'asc';
    } else {
      this._sortBy = key;
      this._sortOrder = 'asc';
    }
    this._page = 1;
  }

  @action
  onPageChange(page) {
    this._page = page;
  }

  @action
  onPageSizeChange(size) {
    this.pageSize = size;
    this._page = 1;
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
