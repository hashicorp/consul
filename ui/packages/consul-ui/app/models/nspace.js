/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';
export const NSPACE_KEY = 'Namespace';

export default class Nspace extends Model {
  @attr('string') uid;
  @attr('string') Name;
  @attr('string') Datacenter;
  @attr('string') Partition;
  // Namespace is the same as Name but please don't alias as we want to keep
  // mutating the response here instead
  @attr('string') Namespace;

  @attr('number') SyncTime;
  @attr('string', { defaultValue: () => '' }) Description;
  @attr({ defaultValue: () => [] }) Resources; // []
  // TODO: Is there some sort of date we can use here
  @attr('string') DeletedAt;
  @attr({
    defaultValue: () => ({
      PolicyDefaults: [],
      RoleDefaults: [],
    }),
  })
  ACLs;
}
