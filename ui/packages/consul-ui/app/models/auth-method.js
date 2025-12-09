/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';
import parse from 'parse-duration';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';

export default class AuthMethod extends Model {
  @attr('string') uid;
  @attr('string') Name;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string', { defaultValue: () => '' }) Description;
  @attr('string', { defaultValue: () => '' }) DisplayName;
  @attr('string', { defaultValue: () => 'local' }) TokenLocality;
  @attr('string') Type;
  @attr() NamespaceRules;
  get MethodName() {
    return this.DisplayName || this.Name;
  }
  @attr() Config;
  @attr('string') MaxTokenTTL;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr() Datacenters; // string[]
  @attr() meta; // {}

  get TokenTTL() {
    return parse(this.MaxTokenTTL);
  }
}
