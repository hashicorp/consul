/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { schedule } from '@ember/runloop';

/**
 * Consul::ListToolbar
 *
 * A generic, presentational toolbar built on the HDS Filter Bar
 * (https://helios.hashicorp.design/components/filter-bar). It translates a
 * page's existing query-param backed filter state (`@filter`, `@search` /
 * `@onsearch`) to and from the Filter Bar's `@filters` shape and `@onFilter`
 * callback, and renders a multi-select group per entry in `@filterGroups`.
 *
 * It performs no data fetching of its own. The set of filters is fully
 * configuration-driven so the same toolbar can back different list pages
 * (services, nodes, ...) which each have a different filter shape.
 *
 * Optional, built-in features:
 *  - a "Search across" dropdown bound to `@filter.searchproperty` (shown
 *    automatically when that filter exists), controlling which properties the
 *    free-text search matches.
 *  - page-specific extras (e.g. quick-filter buttons) via the `:quickFilters`
 *    named block, which yields a small API ({ isSelected, toggleFilterValue,
 *    setFilterValues }) for driving the same `@filter` groups.
 *
 * @argument {object} filter - map of `key -> { value, change, default }` filter
 *   objects, as built by the route template.
 * @argument {string} [search] - the current free-text search term.
 * @argument {Function} [onsearch] - called as `onsearch({ target: { value } })`.
 * @argument {Array} filterGroups - definitions of the multi-select groups. Each
 *   entry: `{ key, text, type?, searchEnabled?, options: [{ value, label }] }`.
 * @argument {string} [searchPlaceholder] - search input placeholder.
 */
export default class ConsulListToolbar extends Component {
  get filterGroups() {
    return this.args.filterGroups || [];
  }

  get searchPlaceholder() {
    return this.args.searchPlaceholder || 'Search';
  }

  get searchProperty() {
    return this.args.filter?.searchproperty;
  }

  // The "Search across" dropdown is only shown when the page actually wires up
  // a `searchproperty` filter with a non-empty set of properties.
  get hasSearchAcross() {
    const searchProperty = this.searchProperty;
    return Boolean(searchProperty && (searchProperty.default || []).length);
  }

  // Whether `value` is currently selected within one of the multi-select
  // filter groups. Exposed to the :quickFilters block to drive pressed states.
  isSelected = (filterKey, value) => {
    return (this.args.filter[filterKey]?.value || []).includes(value);
  };

  // Whether a given search property is currently part of the active search
  // scope, used to drive the checked state of the "Search across" dropdown.
  isSearchProperty = (property) => {
    return (this.searchProperty?.value || []).includes(property);
  };

  // Keep the Filter Bar's applied-filters area expanded at all times so its
  // applied-filter tags (when filters are set) or its "No filters applied"
  // empty state (when none are) stay visible. The HDS Filter Bar collapses
  // that area whenever the filter count drops to zero and exposes no public
  // argument to prevent it, so we hide its toggle button (see the SCSS) and
  // re-expand by clicking that hidden toggle whenever it collapses.
  @action
  setupFilterBar(element) {
    this._filterBarElement = element;
    this.ensureExpanded();
  }

  ensureExpanded = () => {
    const element = this._filterBarElement;
    if (!element) {
      return;
    }
    // Read the toggle state after render so we react to the Filter Bar's own
    // collapse on the latest filter change rather than a stale value.
    schedule('afterRender', () => {
      const toggle = element.querySelector('.hds-filter-bar__applied-filters-toggle-button');
      if (toggle && toggle.getAttribute('aria-expanded') === 'false') {
        toggle.click();
      }
    });
  };

  // Look up the human-readable label for a value within a filter group, used
  // when building the applied-filter tags. Falls back to the raw value.
  labelFor(key, value) {
    const group = this.filterGroups.find((g) => g.key === key);
    const options = group?.options || [];
    return (options.find((o) => o.value === value) || {}).label || value;
  }

  // Maps the page's filter/search state into the object shape the Filter Bar
  // expects for `@filters`. This drives both the applied-filter tags and the
  // checked state of the dropdown options. Only groups with active selections
  // (and a non-empty search term) are included.
  get appliedFilters() {
    const filters = {};

    this.filterGroups.forEach(({ key, text }) => {
      const selected = this.args.filter[key]?.value || [];
      if (selected.length) {
        filters[key] = {
          type: 'multi-select',
          text,
          data: selected.map((value) => ({
            value,
            label: this.labelFor(key, value),
          })),
        };
      }
    });

    if (this.args.search) {
      filters.search = {
        type: 'search',
        text: 'Search',
        data: { value: this.args.search },
      };
    }

    return filters;
  }

  // Receives the full set of applied filters from the Filter Bar whenever the
  // user applies, changes, dismisses or clears a filter (including the search
  // input). Each group is pushed back into the data layer via the `change`
  // action it was given, which expects a comma-joined string in
  // `target.selectedItems` so URL query params keep their `?key=a,b` form. The
  // search term is reported back through `@onsearch`.
  @action
  onFilter(filters) {
    this.filterGroups.forEach(({ key }) => {
      const filter = this.args.filter[key];
      if (!filter) {
        return;
      }
      const next = (filters[key]?.data || []).map((item) => item.value);
      const current = filter.value || [];
      if (current.join(',') !== next.join(',')) {
        filter.change({ target: { selectedItems: next.join(',') } });
      }
    });

    if (this.args.onsearch) {
      const next = filters.search?.data?.value || '';
      if ((this.args.search || '') !== next) {
        this.args.onsearch({ target: { value: next } });
      }
    }

    // The Filter Bar collapses its applied-filters area when the filter count
    // hits zero; re-expand so the "No filters applied" state stays visible.
    this.ensureExpanded();
  }

  // Toggle a single property in the "Search across" scope. At least one
  // property must always remain selected (the search has to look at
  // something), so unchecking the last one is rejected and the checkbox is
  // restored.
  @action
  toggleSearchProperty(property, event) {
    const filter = this.searchProperty;
    const current = filter.value || [];
    let next;
    if (event.target.checked) {
      next = current.includes(property) ? current : [...current, property];
    } else {
      next = current.filter((p) => p !== property);
      if (next.length === 0) {
        event.target.checked = true;
        return;
      }
    }
    filter.change({ target: { selectedItems: next.join(',') } });

    // Changing the search scope re-renders the Filter Bar, which collapses its
    // applied-filters area; re-expand so the tags / "No filters applied" state
    // stay visible.
    this.ensureExpanded();
  }

  // Exposed to the :quickFilters block: replace the full set of selected values
  // for a multi-select group (used by page-specific quick-filter controls that
  // need single-select / exclusive behaviour).
  setFilterValues = (key, values) => {
    const filter = this.args.filter[key];
    if (!filter) {
      return;
    }
    filter.change({ target: { selectedItems: (values || []).join(',') } });
    this.ensureExpanded();
  };

  // Exposed to the :quickFilters block: toggle a single value within a
  // multi-select group, preserving the others.
  toggleFilterValue = (key, value) => {
    const filter = this.args.filter[key];
    if (!filter) {
      return;
    }
    const current = filter.value || [];
    const next = current.includes(value) ? current.filter((v) => v !== value) : [...current, value];
    filter.change({ target: { selectedItems: next.join(',') } });
    this.ensureExpanded();
  };
}
