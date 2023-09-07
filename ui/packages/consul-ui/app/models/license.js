/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uri';

export default class License extends Model {
  @attr('string') uri;
  @attr('boolean') Valid;

  @attr('number') SyncTime;
  @attr() meta; // {}

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;

  @attr() License; // {}
  // @attr() Warnings; // []
}
