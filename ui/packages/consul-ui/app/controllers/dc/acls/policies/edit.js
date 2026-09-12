/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { action } from '@ember/object';

export default class EditController extends Controller {
  @service('form')
  builder;

  @service('dom')
  dom;

  items = [];

  constructor() {
    super(...arguments);
    this.form = this.builder.form('policy');
  }

  setProperties(model) {
    // essentially this replaces the data with changesets
    super.setProperties(
      Object.keys(model).reduce((prev, key, i) => {
        switch (key) {
          case 'item':
            prev[key] = this.form.setData(prev[key]).getData();
            break;
        }
        return prev;
      }, model)
    );
  }

  @action
  change(e, value, item) {
    const event = this.dom.normalizeEvent(e, value);
    const form = this.form;
    try {
      form.handleEvent(event);
    } catch (err) {
      const target = event.target;
      switch (target.name) {
        case 'policy[isScoped]':
          // isScoped is a UI-only toggle — not a real model property.
          // The fieldsets component owns this state, so we silently swallow
          // the "unknown property" error that the form builder would throw.
          break;
        default:
          throw err;
      }
    }
  }

  // Forwarders replacing route-action usage
  @action
  onCreate(item, event) {
    event?.preventDefault();
    this.target.send('create', item, event);
  }

  @action
  onUpdate(item, event) {
    event?.preventDefault();
    this.target.send('update', item, event);
  }

  @action
  onCancel(item, event) {
    event?.preventDefault();
    this.target.send('cancel', item, event);
  }

  @action
  onDelete(item) {
    this.target.send('delete', item);
  }
}
