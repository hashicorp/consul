/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import { MANAGEMENT_ID } from 'consul-ui/models/policy';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'AccessorID';

export default class Token extends Model {
  @attr('string') uid;
  @attr('string') AccessorID;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string') IDPName;
  @attr('string') SecretID;

  @attr('boolean') Legacy;
  @attr('boolean') Local;
  @attr('string', { defaultValue: () => '' }) Description;
  @attr() meta; // {}

  @attr({ defaultValue: () => [] }) Policies;
  @attr({ defaultValue: () => [] }) Roles;
  @attr({ defaultValue: () => [] }) ServiceIdentities;
  @attr({ defaultValue: () => [] }) NodeIdentities;
  @attr('date') CreateTime;
  @attr('string') Hash;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;

  // Legacy
  @attr('string') Type;
  @attr('string', { defaultValue: () => '' }) Name;
  @attr('string') Rules;
  // End Legacy

  @computed('Policies.[]')
  get isGlobalManagement() {
    return (this.Policies || []).find((item) => item.ID === MANAGEMENT_ID);
  }

  @computed('SecretID')
  get hasSecretID() {
    return this.SecretID !== '' && this.SecretID !== '<hidden>';
  }
}
