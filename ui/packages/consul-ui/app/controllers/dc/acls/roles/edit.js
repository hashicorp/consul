/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { inject as service } from '@ember/service';
import Controller from '@ember/controller';
export default class EditController extends Controller {
  @service('form')
  builder;

  init() {
    super.init(...arguments);
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
}
