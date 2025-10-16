/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
import { action } from '@ember/object';

export default class EditController extends Controller {
  @service('form')
  builder;

  constructor() {
    super(...arguments);
    this.form = this.builder.form('role');
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
  delete(item) {
    this.target.send('delete', item);
  }
}
