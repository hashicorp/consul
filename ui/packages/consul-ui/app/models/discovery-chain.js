/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ServiceName';

export default class DiscoveryChain extends Model {
  @attr('string') uid;
  @attr('string') ServiceName;

  @attr('string') Datacenter;
  // Whilst the disco chain itself is scoped to a ns/partition we generally only
  // use data from within the disco chain itself when displaying data (i.e. the
  // configs themselves) We also use the API response JSON for fingerprinting
  // already. All in-all these properties are mainly here for consistency rather
  // than need as of writing in case any assumptions are made expecting disco
  // chain root Partition/Namespace
  @attr('string') Partition;
  @attr('string') Namespace;
  //
  @attr() Chain; // {}
  @attr() meta; // {}
}
