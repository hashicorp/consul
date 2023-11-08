/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';

export default class Certificate extends Model {
  @attr('string') Certificate;
  @attr('number') ExpiresInDays;
}
