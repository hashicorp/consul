/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';
import { inject as service } from '@ember/service';

const modelName = 'service';
export default class ServiceService extends RepositoryService {
  @service store;

  getModelName() {
    return modelName;
  }

  @dataSource('/:partition/:ns/:dc/services')
  async findAllByDatacenter() {
    return super.findAll(...arguments);
  }

  @dataSource('/:partition/:ns/:dc/services/:peer/:peerId')
  async findAllImportedServices(params, configuration) {
    // remember peer.id so that we can add it to to the service later on to setup relationship
    const { peerId } = params;

    // don't send peerId with query
    delete params.peerId;

    // assign the peer as a belongs-to. we don't have access to any information
    // we could use to do this in the serializer so we need to do it manually here
    return super.findAll(params, configuration).then((services) => {
      const peer = this.store.peekRecord('peer', peerId);
      services.forEach((service) => (service.peer = peer));
      return services;
    });
  }

  @dataSource('/:partition/:ns/:dc/gateways/for-service/:gateway')
  findGatewayBySlug(params, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), params);
  }
}
