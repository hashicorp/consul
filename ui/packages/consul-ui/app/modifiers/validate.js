/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { action } from '@ember/object';
import { registerDestructor } from '@ember/destroyable';

class ValidationError extends Error {}

function cleanup(instance) {
  if (instance && instance?.element) {
    instance?.element?.removeEventListener('input', instance?.listen);
    instance?.element?.removeEventListener('blur', instance?.reset);
  }
}

export default class ValidateModifier extends Modifier {
  item = null;
  hash = null;

  validate(value, validations = {}) {
    if (Object.keys(validations).length === 0) {
      return;
    }
    const errors = {};
    Object.entries(this.hash.validations)
      // filter out strings, for now these are helps, but ain't great if someone has a item.help
      .filter(([key, value]) => typeof value !== 'string')
      .forEach(([key, item]) => {
        // optionally set things for you
        if (this.item) {
          this.item[key] = value;
        }
        (item || []).forEach((validation) => {
          const re = new RegExp(validation.test);
          if (!re.test(value)) {
            errors[key] = new ValidationError(validation.error);
          }
        });
      });
    const state = this.hash.chart.state || {};
    if (state.context == null) {
      state.context = {};
    }
    if (Object.keys(errors).length > 0) {
      state.context.errors = errors;
      this.hash.chart.dispatch('ERROR', state.context);
    } else {
      state.context.errors = null;
      this.hash.chart.dispatch('RESET', state.context);
    }
  }

  @action
  reset(e) {
    if (e.target.value.length === 0) {
      const state = this.hash.chart.state;
      if (!state.context) {
        state.context = {};
      }
      if (!state.context.errors) {
        state.context.errors = {};
      }
      Object.entries(this.hash.validations)
        // filter out strings, for now these are helps, but ain't great if someone has a item.help
        .filter(([key, value]) => typeof value !== 'string')
        .forEach(([key, item]) => {
          if (typeof state.context.errors[key] !== 'undefined') {
            delete state.context.errors[key];
          }
        });
      if (Object.keys(state.context.errors).length === 0) {
        state.context.errors = null;
        this.hash.chart.dispatch('RESET', state.context);
      }
    }
  }

  @action
  listen(e) {
    this.validate(e.target.value, this.hash.validations);
  }

  constructor(owner, args) {
    super(owner, args);
    registerDestructor(this, cleanup);
  }

  async modify(element, positional, named) {
    cleanup.call(this);

    this.element = element;
    this.hash = named;
    this.item = positional[0];

    if (typeof this.hash.chart === 'undefined') {
      this.hash.chart = {
        state: {
          context: {},
        },
        dispatch: (state) => {
          switch (state) {
            case 'ERROR':
              this.hash.onchange(this.hash.chart.state.context.errors);
              break;
            case 'RESET':
              this.hash.onchange();
              break;
          }
        },
      };
    }

    this.element.addEventListener('input', this.listen);
    this.element.addEventListener('blur', this.reset);

    if (this.element.value.length > 0) {
      await Promise.resolve();
      if (this && this.element) {
        this.validate(this.element.value, this.hash.validations);
      }
    }
  }
}
