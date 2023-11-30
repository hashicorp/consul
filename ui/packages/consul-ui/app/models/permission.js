/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';

export default class Permission extends Model {
  @attr('string') Resource;
  @attr('string') Segment;
  @attr('string') Access;
  @attr('boolean') Allow;
}
