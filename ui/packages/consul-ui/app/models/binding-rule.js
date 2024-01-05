/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class BindingRule extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string', { defaultValue: () => '' }) Description;
  @attr('string') AuthMethod;
  @attr('string', { defaultValue: () => '' }) Selector;
  @attr('string') BindType;
  @attr('string') BindName;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
}
