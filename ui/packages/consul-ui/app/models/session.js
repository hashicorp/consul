/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import { nullValue } from 'consul-ui/decorators/replace';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class Session extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Name;
  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string') Node;
  @attr('string') Behavior;
  @attr('string') TTL;
  @attr('number') LockDelay;
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;

  @nullValue([]) @attr({ defaultValue: () => [] }) NodeChecks;
  @nullValue([]) @attr({ defaultValue: () => [] }) ServiceChecks;

  @attr({ defaultValue: () => [] }) Resources; // []

  @computed('NodeChecks', 'ServiceChecks')
  get checks() {
    return [...this.NodeChecks, ...this.ServiceChecks.map(({ ID }) => ID)];
  }
}
