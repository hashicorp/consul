import Serializer from 'ember-data/serializers/rest';

import { set } from '@ember/object';
import {
  HEADERS_SYMBOL as HTTP_HEADERS_SYMBOL,
  HEADERS_INDEX as HTTP_HEADERS_INDEX,
} from 'consul-ui/utils/http/consul';
export default Serializer.extend({
  // this could get confusing if you tried to override
  // say `normalizeQueryResponse`
  // TODO: consider creating a method for each one of the `normalize...Response` family
  normalizeResponse: function(store, primaryModelClass, payload, id, requestType) {
    // Pick the meta/headers back off the payload and cleanup
    // before we go through serializing
    const headers = payload[HTTP_HEADERS_SYMBOL] || {};
    delete payload[HTTP_HEADERS_SYMBOL];
    const normalizedPayload = this.normalizePayload(payload, id, requestType);
    // put the meta onto the response, here this is ok
    // as JSON-API allows this and our specific data is now in
    // response[primaryModelClass.modelName]
    // so we aren't in danger of overwriting anything
    // (which was the reason for the Symbol-like property earlier)
    // use a method modelled on ember-data methods so we have the opportunity to
    // do this on a per-model level
    const meta = this.normalizeMeta(
      store,
      primaryModelClass,
      headers,
      normalizedPayload,
      id,
      requestType
    );
    if (requestType === 'queryRecord') {
      normalizedPayload.meta = meta;
    }
    return this._super(
      store,
      primaryModelClass,
      {
        meta: meta,
        [primaryModelClass.modelName]: normalizedPayload,
      },
      id,
      requestType
    );
  },
  timestamp: function() {
    return new Date().getTime();
  },
  normalizeMeta: function(store, primaryModelClass, headers, payload, id, requestType) {
    const meta = {
      cursor: headers[HTTP_HEADERS_INDEX],
    };
    if (requestType === 'query') {
      meta.date = this.timestamp();
      payload.forEach(function(item) {
        set(item, 'SyncTime', meta.date);
      });
    }
    return meta;
  },
  normalizePayload: function(payload, id, requestType) {
    return payload;
  },
});
