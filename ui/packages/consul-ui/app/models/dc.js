/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const FOREIGN_KEY = 'Datacenter';
export const SLUG_KEY = 'Name';

export default class Datacenter extends Model {
  @attr('string') uri;
  @attr('string') Name;
  // autopilot/state
  @attr('boolean') Healthy;
  @attr('number') FailureTolerance;
  @attr('number') OptimisticFailureTolerance;
  @attr('string') Leader;
  @attr() Voters; // []
  @attr() Servers; // [] the API uses {} but we reshape that on the frontend
  @attr() RedundancyZones;
  @attr() Default; // added by the frontend, {Servers: []} any server that isn't in a zone
  @attr() ReadReplicas;
  //
  @attr('boolean') Local;
  @attr('boolean') Primary;
  @attr('string') DefaultACLPolicy;

  @attr('boolean', { defaultValue: () => true }) MeshEnabled;
}
