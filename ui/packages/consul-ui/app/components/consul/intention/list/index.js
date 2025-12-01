/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class ConsulIntentionList extends Component {
  @service('repository/intention') repo;

  get items() {
    return this.args.items || [];
  }

  get isManagedByCRDs() {
    // Automatically recompute when items change
    // Access this.items to establish reactivity dependency
    if (this.items) {
      return this.repo.isManagedByCRDs();
    }
    return false;
  }

  get checkedItem() {
    if (this.items.length === 1 && this.args.check) {
      return this.items[0].SourceName === this.args.check ? this.items[0] : null;
    }
    return null;
  }
}
