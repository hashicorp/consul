/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/topology';
import { inject as service } from '@ember/service';

export default class TopologySerializer extends Serializer {
  @service('store') store;

  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForQueryRecord(respond, query) {
    const intentionSerializer = this.store.serializerFor('intention');
    return super.respondForQueryRecord(function (cb) {
      return respond(function (headers, body) {
        body.Downstreams.forEach((item) => {
          item.Intention.SourceName = item.Name;
          item.Intention.SourceNS = item.Namespace;
          item.Intention.DestinationName = query.id;
          item.Intention.DestinationNS = query.ns || 'default';
          intentionSerializer.ensureID(item.Intention);
        });
        body.Upstreams.forEach((item) => {
          item.Intention.SourceName = query.id;
          item.Intention.SourceNS = query.ns || 'default';
          item.Intention.DestinationName = item.Name;
          item.Intention.DestinationNS = item.Namespace;
          intentionSerializer.ensureID(item.Intention);
        });
        return cb(headers, {
          ...body,
          [SLUG_KEY]: query.id,
        });
      });
    }, query);
  }
}
