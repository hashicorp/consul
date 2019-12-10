import Serializer from './http';

import { set } from '@ember/object';
import {
  HEADERS_SYMBOL as HTTP_HEADERS_SYMBOL,
  HEADERS_INDEX as HTTP_HEADERS_INDEX,
  HEADERS_DATACENTER as HTTP_HEADERS_DATACENTER,
} from 'consul-ui/utils/http/consul';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import createFingerprinter from 'consul-ui/utils/create-fingerprinter';

const map = function(obj, cb) {
  if (!Array.isArray(obj)) {
    return [obj].map(cb)[0];
  }
  return obj.map(cb);
};

const attachHeaders = function(headers, body, query = {}) {
  // lowercase everything incase we get browser inconsistencies
  const lower = {};
  Object.keys(headers).forEach(function(key) {
    lower[key.toLowerCase()] = headers[key];
  });
  // Add a 'pretend' Datacenter header, they are not headers
  // the come from the request but we add them here so we can use them later
  // for store reconciliation
  if (typeof query.dc !== 'undefined') {
    lower[HTTP_HEADERS_DATACENTER.toLowerCase()] = query.dc;
  }
  body[HTTP_HEADERS_SYMBOL] = lower;
  return body;
};

export default Serializer.extend({
  fingerprint: createFingerprinter(DATACENTER_KEY),
  respondForQuery: function(respond, query) {
    return respond((headers, body) =>
      attachHeaders(
        headers,
        map(body, this.fingerprint(this.primaryKey, this.slugKey, query.dc)),
        query
      )
    );
  },
  respondForQueryRecord: function(respond, query) {
    return respond((headers, body) =>
      attachHeaders(
        headers,
        map(body, this.fingerprint(this.primaryKey, this.slugKey, query.dc)),
        query
      )
    );
  },
  respondForCreateRecord: function(respond, serialized, data) {
    const slugKey = this.slugKey;
    const primaryKey = this.primaryKey;
    return respond((headers, body) => {
      // If creates are true use the info we already have
      if (body === true) {
        body = data;
      }
      // Creates need a primaryKey adding
      return this.fingerprint(primaryKey, slugKey, data[DATACENTER_KEY])(body);
    });
  },
  respondForUpdateRecord: function(respond, serialized, data) {
    const slugKey = this.slugKey;
    const primaryKey = this.primaryKey;
    return respond((headers, body) => {
      // If updates are true use the info we already have
      if (body === true) {
        body = data;
      }
      return this.fingerprint(primaryKey, slugKey, data[DATACENTER_KEY])(body);
    });
  },
  respondForDeleteRecord: function(respond, serialized, data) {
    const slugKey = this.slugKey;
    const primaryKey = this.primaryKey;
    return respond((headers, body) => {
      // Deletes only need the primaryKey/uid returning
      return {
        [primaryKey]: this.fingerprint(primaryKey, slugKey, data[DATACENTER_KEY])({
          [slugKey]: data[slugKey],
        })[primaryKey],
      };
    });
  },
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
      dc: headers[HTTP_HEADERS_DATACENTER.toLowerCase()],
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
