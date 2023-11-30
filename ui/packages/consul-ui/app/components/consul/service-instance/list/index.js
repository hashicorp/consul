/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class ServiceInstanceList extends Component {
  get areAllExternalSourcesMatching() {
    const firstSource = this.args.items[0]?.Service?.Meta?.['external-source'];

    const matching = this.args.items.every(
      (instance) => instance.Service?.Meta?.['external-source'] === firstSource
    );
    return matching;
  }
}
