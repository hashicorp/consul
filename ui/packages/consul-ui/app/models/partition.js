/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';
export const PARTITION_KEY = 'Partition';

export default class PartitionModel extends Model {
  @attr('string') uid;
  @attr('string') Name;
  @attr('string') Description;
  // TODO: Is there some sort of date we can use here
  @attr('string') DeletedAt;
  @attr('string') Datacenter;

  @attr('string') Namespace; // always ""
  // Partition is the same as Name but please don't alias as we want to keep
  // mutating the response here instead
  @attr('string') Partition;

  @attr('number') SyncTime;
  @attr() meta;
}
