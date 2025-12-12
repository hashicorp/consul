/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class DatacenterSelectorComponent extends Component {
  @tracked search = '';

  get filteredItems() {
    const lowerCaseSearch = this.search.toLowerCase();
    return this.args.dcs.filter((dc) => dc.Name.toLowerCase().includes(lowerCaseSearch));
  }

  @action
  onSearchInput(e) {
    this.search = e.target.value;
  }
}
