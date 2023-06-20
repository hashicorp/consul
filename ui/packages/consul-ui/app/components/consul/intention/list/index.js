/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class ConsulIntentionList extends Component {
  @service('repository/intention') repo;

  @tracked isManagedByCRDs;

  constructor(owner, args) {
    super(...arguments);
    this.updateCRDManagement(args.items);
  }
  get items() {
    return this.args.items || [];
  }
  get checkedItem() {
    if (this.items.length === 1 && this.args.check) {
      return this.items[0].SourceName === this.args.check ? this.items[0] : null;
    }
    return null;
  }
  @action
  updateCRDManagement() {
    this.isManagedByCRDs = this.repo.isManagedByCRDs();
  }
}
