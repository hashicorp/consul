/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from 'consul-ui/routing/route';
import { assert } from '@ember/debug';
import { Promise, hash } from 'rsvp';
export default Route.extend({
  // repo: service('repositoryName'),
  isCreate: function (params, transition) {
    return transition.targetName.split('.').pop() === 'create';
  },
  model: function (params, transition) {
    const repo = this.repo;
    assert(
      "`repo` is undefined, please define RepositoryService using `repo: service('repositoryName')`",
      typeof repo !== 'undefined'
    );
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.optionalParams().nspace;
    const partition = this.optionalParams().partition;
    const create = this.isCreate(...arguments);
    return hash({
      dc: dc,
      partition: partition,
      nspace: nspace,
      create: create,
      ...repo.status({
        item: create
          ? Promise.resolve(
              repo.create({
                Datacenter: dc,
                Namespace: nspace,
                Partition: partition,
              })
            )
          : repo.findBySlug({
              partition: partition,
              ns: nspace,
              dc: dc,
              id: params.id,
            }),
      }),
    });
  },
});
