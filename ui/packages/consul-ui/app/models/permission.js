/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Model, { attr } from '@ember-data/model';

export default class Permission extends Model {
  @attr('string') Resource;
  @attr('string') Segment;
  @attr('string') Access;
  @attr('boolean') Allow;
}
