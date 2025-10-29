/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class EditController extends Controller {
  @service('dom') dom;
  @service('form') builder;

  isScoped = false;

  constructor() {
    super(...arguments);
    this.form = this.builder.form('token');
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
        default:
          throw err;
      }
    }
  }

  @action
  use(item) {
    this.target.send('use', item);
  }

  @action onCreate(item, event) {
    event?.preventDefault();
    this.target.send('create', item, event);
  }

  @action onUpdate(item, event) {
    event?.preventDefault();
    this.target.send('update', item, event);
  }

  @action onCancel(item, event) {
    event?.preventDefault();
    this.target.send('cancel', item, event);
  }

  @action onDelete(item) {
    this.target.send('delete', item);
  }
}
