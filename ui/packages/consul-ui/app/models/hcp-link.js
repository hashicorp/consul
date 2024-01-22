/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';

export default class HcpLink extends Model {
  @attr() status;

  @computed('status')
  get isLinked() {
    return (
      (this.status['consul.io/hcp/link']['conditions'] || []).filter(
        (condition) => condition.type === 'linked' && condition.status === 'STATUS_TRUE'
      ).length > 0
    );
  }
}
