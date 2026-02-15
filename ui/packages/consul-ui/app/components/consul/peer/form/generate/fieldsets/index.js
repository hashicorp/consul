/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class PeerGenerateFieldSets extends Component {
  @action
  onInput(addresses) {
    if (addresses) {
      addresses = addresses.split(',').map(address => address.trim());
    } else {
      addresses = [];
    }

    this.args.item.ServerExternalAddresses = addresses;
  } 
}
