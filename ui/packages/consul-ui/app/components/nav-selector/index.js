/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class NavSelectorComponent extends Component {
  @tracked search = '';

  get filteredItems() {
    const lowerCaseSearch = this.search.toLowerCase();

    if (lowerCaseSearch) {
      return this.args.items.filter((item) =>
        item[this.args.key].toLowerCase().includes(lowerCaseSearch)
      );
    } else {
      return this.args.items;
    }
  }

  @action
  onSearchInput(e) {
    this.search = e.target.value;
  }
}
