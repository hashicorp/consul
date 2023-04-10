/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { set, action } from '@ember/object';

export default class TopologyRoute extends Route {
  @service('data-source/service') data;
  @service('repository/intention') repo;
  @service('feedback') feedback;

  @action
  async createIntention(source, destination) {
    // begin with a create action as it makes more sense if the we can't even
    // get a list of intentions
    let notification = this.feedback.notification('create', 'intention');
    try {
      // intentions will be a proxy object
      let intentions = await this.intentions;
      let intention = intentions.find((item) => {
        return (
          item.Datacenter === source.Datacenter &&
          item.SourceName === source.Name &&
          item.SourceNS === source.Namespace &&
          item.SourcePartition === source.Partition &&
          item.DestinationName === destination.Name &&
          item.DestinationNS === destination.Namespace &&
          item.DestinationPartition === destination.Partition
        );
      });
      if (typeof intention === 'undefined') {
        intention = this.repo.create({
          Datacenter: source.Datacenter,
          SourceName: source.Name,
          SourceNS: source.Namespace || 'default',
          SourcePartition: source.Partition || 'default',
          DestinationName: destination.Name,
          DestinationNS: destination.Namespace || 'default',
          DestinationPartition: destination.Partition || 'default',
        });
      } else {
        // we found an intention in the find higher up, so we are updating
        notification = this.feedback.notification('update', 'intention');
      }
      set(intention, 'Action', 'allow');
      await this.repo.persist(intention);
      notification.success(intention);
    } catch (e) {
      notification.error(e);
    }
    this.refresh();
  }

  afterModel(model, transition) {
    const params = {
      ...this.optionalParams(),
      ...this.paramsFor('dc'),
      ...this.paramsFor('dc.services.show'),
    };
    this.intentions = this.data.source(
      (uri) =>
        uri`/${params.partition}/${params.nspace}/${params.dc}/intentions/for-service/${params.name}`
    );
  }

  async deactivate(transition) {
    const intentions = await this.intentions;
    intentions.destroy();
  }
}
