/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Serializer from './application';
import { EmbeddedRecordsMixin } from '@ember-data/serializer/rest';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/node';
import { classify } from '@ember/string';

// TODO: Looks like ID just isn't used at all consider just using .Node for
// the SLUG_KEY
const fillSlug = function (item) {
  if (item[SLUG_KEY] === '') {
    item[SLUG_KEY] = item['Node'];
  }
  return item;
};

export default class NodeSerializer extends Serializer.extend(EmbeddedRecordsMixin) {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  attrs = {
    Services: {
      embedded: 'always',
    },
  };

  transformHasManyResponse(store, relationship, item, parent = null) {
    let checks = {};
    let serializer;
    switch (relationship.key) {
      case 'Services':
        (item.Checks || [])
          .filter((item) => {
            return item.ServiceID !== '';
          })
          .forEach((item) => {
            if (typeof checks[item.ServiceID] === 'undefined') {
              checks[item.ServiceID] = [];
            }
            checks[item.ServiceID].push(item);
          });
        if (item.PeerName === '') {
          item.PeerName = undefined;
        }
        serializer = this.store.serializerFor(relationship.type);
        item.Services = item.Services.map((service) =>
          serializer.transformHasManyResponseFromNode(item, service, checks)
        );
        return item;
    }
    return super.transformHasManyResponse(...arguments);
  }

  respondForQuery(respond, query, data, modelClass) {
    const body = super.respondForQuery(
      (cb) => respond((headers, body) => cb(headers, body.map(fillSlug))),
      query
    );
    modelClass.eachRelationship((key, relationship) => {
      body.forEach((item) =>
        this[`transform${classify(relationship.kind)}Response`](
          this.store,
          relationship,
          item,
          body
        )
      );
    });
    return body;
  }

  respondForQueryRecord(respond, query, data, modelClass) {
    const body = super.respondForQueryRecord(
      (cb) =>
        respond((headers, body) => {
          return cb(headers, fillSlug(body));
        }),
      query
    );

    modelClass.eachRelationship((key, relationship) => {
      this[`transform${classify(relationship.kind)}Response`](this.store, relationship, body);
    });
    return body;
  }

  respondForQueryLeader(respond, query) {
    // don't call super here we don't care about
    // ids/fingerprinting
    return respond((headers, body) => {
      // This response/body is just an ip:port like `"10.0.0.1:8500"`
      // split it and make it look like a `C`onsul.`R`esponse
      // popping off the end for ports should cover us for IPv6 addresses
      // as we should always get a `address:port` or `[a:dd:re:ss]:port` combo
      const temp = body.split(':');
      const port = temp.pop();
      const address = temp.join(':');
      return this.attachHeaders(
        headers,
        {
          Address: address,
          Port: port,
        },
        query
      );
    });
  }
}
