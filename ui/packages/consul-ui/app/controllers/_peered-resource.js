/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';

export default class PeeredResourceController extends Controller {
  @service abilities;

  get _searchProperties() {
    const { searchProperties } = this;

    if (!this.abilities.can('use peers')) {
      return searchProperties.filter((propertyName) => propertyName !== 'PeerName');
    } else {
      return searchProperties;
    }
  }
}
