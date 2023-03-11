/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Node';

export default class Coordinate extends Model {
  @attr('string') uid;
  @attr('string') Node;

  @attr() Coord; // {Vec, Error, Adjustment, Height}
  @attr('string') Segment;
  @attr('string') Datacenter;
  @attr('string') Partition;
  @attr('number') SyncTime;
}
