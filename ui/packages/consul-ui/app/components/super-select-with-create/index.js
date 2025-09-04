/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { get, set, computed } from '@ember/object';
import { inject as service } from '@ember/service';
import { next } from '@ember/runloop';
import { task } from 'ember-concurrency';
import { resolve } from 'rsvp';
import { filterOptions, defaultMatcher } from 'ember-power-select/utils/group-utils';

export default Component.extend({
  tagName: '',

  // PowerSelectWithCreate compatible props
  options: null,
  selected: null,
  onCreate: function () {},
  onChange: function () {},
  onFilter: function () {},
  onOpen: function () {},
  onClose: function () {},
  buildSuggestion: null,
  showCreateWhen: null,
  showCreatePosition: 'top',
  searchField: 'Name',
  searchPlaceholder: 'Search...',
  disabled: false,

  // HDS SuperSelect props
  label: '',
  listboxAriaLabel: 'Available options',
  isOptional: true,
  helperText: '',
  errorText: '',

  dom: service('dom'),
  matcher: defaultMatcher,

  // Internal state
  searchTerm: '',
  isCreating: false,

  init() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
    set(this, 'searchTerm', '');
    set(this, 'isCreating', false);

    next(this, this.setupAriaFixes);
  },

  didInsertElement() {
    this._super(...arguments);
    this.setupKeyboardHandlers();
  },

  willDestroyElement() {
    this._super(...arguments);
    this._listeners.remove();
    this.teardownAriaFixes();
    this.teardownKeyboardHandlers();
  },

  // Filter options using PowerSelectWithCreate logic
  filterOptions(options, searchText) {
    let matcher;
    if (get(this, 'searchField')) {
      matcher = (option, text) => this.matcher(get(option, get(this, 'searchField')), text);
    } else {
      matcher = (option, text) => this.matcher(option, text);
    }
    return filterOptions(options || [], searchText, matcher);
  },

  // Check if we should show create option - works with existing parent actions
  shouldShowCreateOption(term, filteredOptions) {
    const showCreateWhen = get(this, 'showCreateWhen');
    if (showCreateWhen) {
      // The parent's isUnique action expects (items, term)
      // showCreateWhen is bound as: {{action "isUnique" services}}
      // So when we call showCreateWhen(term), it becomes isUnique(services, term)
      return showCreateWhen(term);
    }
    return true;
  },

  // Add create option to results
  addCreateOption(term, results) {
    if (this.shouldShowCreateOption(term, results)) {
      const suggestion = this.buildSuggestionForTerm(term);

      if (get(this, 'showCreatePosition') === 'bottom') {
        results.push(suggestion);
      } else {
        results.unshift(suggestion);
      }
    }
  },

  // Build suggestion object
  buildSuggestionForTerm(term) {
    return {
      __isSuggestion__: true,
      __value__: term,
      text: this.buildSuggestionLabel(term),
      [get(this, 'searchField')]: this.buildSuggestionLabel(term),
    };
  },

  // Build suggestion label - works with existing bound actions
  buildSuggestionLabel(term) {
    const buildSuggestion = get(this, 'buildSuggestion');
    if (buildSuggestion) {
      // The parent's action is bound as: {{action "createNewLabel" "Use a Consul Service called '{{term}}'"}}
      // When we call buildSuggestion(term), it becomes createNewLabel("Use a Consul Service called '{{term}}'", term)
      return buildSuggestion(term);
    }
    return `Add "${term}"...`;
  },

  // Enhanced options that include search results + create option
  enhancedOptions: computed('options.[]', 'searchTerm', function () {
    const options = get(this, 'options') || [];
    const searchTerm = get(this, 'searchTerm') || '';

    if (searchTerm.length === 0) {
      return options;
    }

    // Filter existing options
    let filteredOptions = this.filterOptions(options, searchTerm);

    // Add create option if conditions are met
    this.addCreateOption(searchTerm, filteredOptions);

    return filteredOptions;
  }),

  // Create new option task
  createOption: task(function* (createValue) {
    set(this, 'isCreating', true);

    try {
      const onCreate = get(this, 'onCreate');

      // For drop-in compatibility, onCreate is likely bound to onchange action
      // which expects the property name and value
      // So onCreate is: {{action onchange "SourceName"}}
      // We need to create a service object and pass it to the bound action

      let newOption = {
        [get(this, 'searchField')]: createValue,
      };

      // Call onCreate - this will set the property in the parent
      if (onCreate) {
        // onCreate is bound as {{action onchange "SourceName"}}
        // So calling onCreate(newOption) becomes onchange("SourceName", newOption)
        yield onCreate(newOption);
      }

      // Add the new option to the options array so it appears in the list
      const options = get(this, 'options') || [];
      options.pushObject(newOption);

      // The onChange will be called automatically by onCreate since it's the same action
      // But we ensure the selection happens
      const onChange = get(this, 'onChange');
      if (onChange && onChange !== onCreate) {
        next(this, () => {
          onChange(newOption);
        });
      }

      return newOption;
    } catch (error) {
      console.error('Error creating option:', error);
      throw error;
    } finally {
      set(this, 'isCreating', false);
      set(this, 'searchTerm', '');
    }
  }),

  // Keyboard support
  setupKeyboardHandlers() {
    const element = this.element;
    if (element) {
      this.keydownHandler = (event) => {
        if (event.key === 'Enter') {
          const searchTerm = get(this, 'searchTerm');
          if (searchTerm && searchTerm.trim()) {
            const dropdown = element.querySelector('[role="listbox"]');
            const highlightedOption =
              dropdown && dropdown.querySelector('[aria-selected="true"], .is-highlighted');

            if (highlightedOption && highlightedOption.querySelector('.create-option')) {
              event.preventDefault();
              event.stopPropagation();
              this.createOption.perform(searchTerm.trim());
            }
          }
        }
      };

      element.addEventListener('keydown', this.keydownHandler);
    }
  },

  teardownKeyboardHandlers() {
    const element = this.element;
    if (element && this.keydownHandler) {
      element.removeEventListener('keydown', this.keydownHandler);
    }
  },

  // ARIA fixes (keeping existing implementation)
  setupAriaFixes() {
    this.ariaObserver = new MutationObserver(() => {
      this.fixAriaIssues();
    });

    const element = this.element || document.querySelector('.super-select-with-create');
    if (element) {
      this.ariaObserver.observe(element, {
        childList: true,
        subtree: true,
        attributes: true,
        attributeFilter: ['role', 'aria-selected', 'aria-expanded', 'aria-controls'],
      });

      setTimeout(() => this.fixAriaIssues(), 100);
    }
  },

  teardownAriaFixes() {
    if (this.ariaObserver) {
      this.ariaObserver.disconnect();
      this.ariaObserver = null;
    }
  },

  fixAriaIssues() {
    const element = this.element || document.querySelector('.super-select-with-create');
    if (!element) return;

    element.querySelectorAll('[role="alert"][aria-selected]').forEach((el) => {
      el.setAttribute('role', 'option');
    });

    element.querySelectorAll('[aria-controls]').forEach((el) => {
      const controlsId = el.getAttribute('aria-controls');
      const dropdown = document.getElementById(controlsId);

      if (!dropdown) {
        el.removeAttribute('aria-controls');
      } else if (el.getAttribute('role') === 'combobox' && !el.hasAttribute('aria-expanded')) {
        el.setAttribute('aria-expanded', dropdown.offsetParent !== null ? 'true' : 'false');
      }
    });

    element.querySelectorAll('[role="listbox"]').forEach((listbox) => {
      if (!listbox.hasAttribute('aria-label') && !listbox.hasAttribute('aria-labelledby')) {
        listbox.setAttribute('aria-label', get(this, 'listboxAriaLabel') || 'Available options');
      }
    });

    element.querySelectorAll('[role="listbox"]:not([tabindex])').forEach((listbox) => {
      listbox.setAttribute('tabindex', '-1');
    });
  },

  actions: {
    handleFilter(term) {
      set(this, 'searchTerm', term);
      next(this, this.fixAriaIssues);

      const onFilter = get(this, 'onFilter');
      if (onFilter) {
        onFilter(term);
      }
    },

    handleChange(selectedOption) {
      if (selectedOption && get(selectedOption, '__isSuggestion__')) {
        const createValue = get(selectedOption, '__value__');
        this.createOption.perform(createValue);
      } else {
        const onChange = get(this, 'onChange');
        if (onChange) {
          onChange(selectedOption);
        }
      }
    },

    handleOpen() {
      next(this, this.fixAriaIssues);

      const onOpen = get(this, 'onOpen');
      if (onOpen) {
        onOpen();
      }
    },

    handleClose() {
      set(this, 'searchTerm', '');

      const onClose = get(this, 'onClose');
      if (onClose) {
        onClose();
      }
    },
  },
});
