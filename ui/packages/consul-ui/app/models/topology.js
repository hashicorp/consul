/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ServiceName';

export default class Topology extends Model {
  @attr('string') uid;
  @attr('string') ServiceName;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string') Protocol;
  @attr('boolean') FilteredByACLs;
  @attr('boolean') TransparentProxy;
  @attr('boolean') ConnectNative;
  @attr() Upstreams; // Service[]
  @attr() Downstreams; // Service[],
  @attr() meta; // {}

  @computed('Downstreams')
  get notDefinedIntention() {
    let undefinedDownstream = false;

    undefinedDownstream =
      this.Downstreams.filter(
        (item) =>
          item.Source === 'specific-intention' &&
          !item.TransparentProxy &&
          !item.ConnectNative &&
          item.Intention.Allowed
      ).length !== 0;

    return undefinedDownstream;
  }

  @computed('Downstreams', 'Upstreams')
  // A service has a wildcard intention if `Allowed == true`  and `HasExact = false`
  // The Permissive Intention notice appears if at least one upstream or downstream has
  // a wildcard intention
  get wildcardIntention() {
    const downstreamWildcard =
      this.Downstreams.filter((item) => !item.Intention.HasExact && item.Intention.Allowed)
        .length !== 0;

    const upstreamWildcard =
      this.Upstreams.filter((item) => !item.Intention.HasExact && item.Intention.Allowed).length !==
      0;

    return downstreamWildcard || upstreamWildcard;
  }

  @computed('Downstreams', 'Upstreams')
  get noDependencies() {
    return this.Upstreams.length === 0 && this.Downstreams.length === 0;
  }
}
