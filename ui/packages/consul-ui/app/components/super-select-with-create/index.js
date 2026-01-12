/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { get, set, computed } from '@ember/object';
import { inject as service } from '@ember/service';
import { task } from 'ember-concurrency';

export default Component.extend({
  tagName: '',

  // Props
  options: null,
  selected: null,
  onChange() {},
  onFilter() {},
  onOpen() {},
  onClose() {},
  buildSuggestion: null,
  showCreateWhen: null,
  showCreatePosition: 'top',
  searchField: 'Name',
  searchPlaceholder: 'Search...',
  disabled: false,
  label: '',
  listboxAriaLabel: 'Available options',
  isOptional: true,
  helperText: '',
  errorText: '',

  dom: service(),

  // State
  searchTerm: '',
  isCreating: false,

  init() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
    set(this, 'searchTerm', '');
    set(this, 'isCreating', false);
    this.customSearch = this.customSearch.bind(this);
  },

  willDestroyElement() {
    this._super(...arguments);
    this._listeners.remove();
  },

  allOptions: computed('options.[]', function () {
    return this.options || [];
  }),

  customSearch(term, options) {
    // Ignore the passed options parameter and use our own
    const allOptions = this.options || [];

    if (!term || term.trim() === '') {
      return allOptions;
    }

    const filtered = this.filterOptions(allOptions, term);

    // Check if there's an exact match
    const hasExactMatch = filtered.some((option) => {
      const optionValue = this.searchField ? get(option, this.searchField) : option;
      return optionValue && optionValue.toLowerCase() === term.toLowerCase();
    });

    if (!hasExactMatch && this.shouldShowCreateOption(term)) {
      const suggestion = this.buildSuggestionForTerm(term);
      this.showCreatePosition === 'bottom'
        ? filtered.push(suggestion)
        : filtered.unshift(suggestion);
    }

    return filtered;
  },

  filterOptions(options, term) {
    // Add safety check to ensure options is an array
    if (!Array.isArray(options)) {
      return [];
    }

    return options.filter((option) => {
      const optionValue = this.searchField ? get(option, this.searchField) : option;
      return optionValue && optionValue.toLowerCase().includes(term.toLowerCase());
    });
  },

  shouldShowCreateOption(term) {
    // Only hide create option if there's an exact match
    const allOptions = this.options || [];
    const hasExactMatch = allOptions.some((option) => {
      const optionValue = this.searchField ? get(option, this.searchField) : option;
      return optionValue && optionValue.toLowerCase() === term.toLowerCase();
    });
    return !hasExactMatch;
  },

  buildSuggestionForTerm(term) {
    const buildSuggestion = this.buildSuggestion;

    let label;
    if (buildSuggestion) {
      // Replace __TERM__ placeholder with actual term
      label = buildSuggestion.replace('__TERM__', term);
    } else {
      label = `Add "${term}"...`;
    }

    return {
      __isSuggestion__: true,
      __value__: term,
      text: label,
      [this.searchField]: label,
    };
  },

  createOption: task(function* (value) {
    set(this, 'isCreating', true);
    try {
      const newOption = { [this.searchField]: value };
      // Add to options array first (optimistic update)
      (this.options || []).pushObject(newOption);
      // Call onChange handler for both creation and selection
      yield this.onChange(newOption);
      return newOption;
    } finally {
      set(this, 'isCreating', false);
      set(this, 'searchTerm', '');
    }
  }),

  actions: {
    handleFilter(term) {
      set(this, 'searchTerm', term);
      if (this.onFilter) this.onFilter(term);
    },

    handleChange(option) {
      option?.__isSuggestion__
        ? this.createOption.perform(option.__value__)
        : this.onChange(option);
    },

    handleOpen() {
      if (this.onOpen) this.onOpen();
    },

    handleClose() {
      set(this, 'searchTerm', '');
      if (this.onClose) this.onClose();
    },
  },
});
