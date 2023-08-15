/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Serializer from './http';
import { set } from '@ember/object';

import {
  HEADERS_SYMBOL as HTTP_HEADERS_SYMBOL,
  HEADERS_INDEX as HTTP_HEADERS_INDEX,
  HEADERS_DATACENTER as HTTP_HEADERS_DATACENTER,
  HEADERS_NAMESPACE as HTTP_HEADERS_NAMESPACE,
  HEADERS_PARTITION as HTTP_HEADERS_PARTITION,
} from 'consul-ui/utils/http/consul';
import { CACHE_CONTROL as HTTP_HEADERS_CACHE_CONTROL } from 'consul-ui/utils/http/headers';
import { FOREIGN_KEY as DATACENTER_KEY } from 'consul-ui/models/dc';
import { NSPACE_KEY } from 'consul-ui/models/nspace';
import { PARTITION_KEY } from 'consul-ui/models/partition';
import createFingerprinter from 'consul-ui/utils/create-fingerprinter';

const map = function (obj, cb) {
  if (!Array.isArray(obj)) {
    return [obj].map(cb)[0];
  }
  return obj.map(cb);
};

const attachHeaders = function (headers, body, query = {}) {
  // lowercase everything incase we get browser inconsistencies
  const lower = {};
  Object.keys(headers).forEach(function (key) {
    lower[key.toLowerCase()] = headers[key];
  });
  //
  body[HTTP_HEADERS_SYMBOL] = lower;
  return body;
};
export default class ApplicationSerializer extends Serializer {
  attachHeaders = attachHeaders;
  fingerprint = createFingerprinter(DATACENTER_KEY, NSPACE_KEY, PARTITION_KEY);

  respondForQuery(respond, query) {
    return respond((headers, body) =>
      attachHeaders(
        headers,
        map(
          body,
          this.fingerprint(
            this.primaryKey,
            this.slugKey,
            query.dc,
            headers[HTTP_HEADERS_NAMESPACE],
            headers[HTTP_HEADERS_PARTITION]
          )
        ),
        query
      )
    );
  }

  respondForQueryRecord(respond, query) {
    return respond((headers, body) =>
      attachHeaders(
        headers,
        this.fingerprint(
          this.primaryKey,
          this.slugKey,
          query.dc,
          headers[HTTP_HEADERS_NAMESPACE],
          headers[HTTP_HEADERS_PARTITION]
        )(body),
        query
      )
    );
  }

  respondForCreateRecord(respond, serialized, data) {
    const slugKey = this.slugKey;
    const primaryKey = this.primaryKey;

    return respond((headers, body) => {
      // If creates are true use the info we already have
      if (body === true) {
        body = data;
      }
      // Creates need a primaryKey adding
      return this.fingerprint(
        primaryKey,
        slugKey,
        data[DATACENTER_KEY],
        headers[HTTP_HEADERS_NAMESPACE],
        data.Partition
      )(body);
    });
  }

  respondForUpdateRecord(respond, serialized, data) {
    const slugKey = this.slugKey;
    const primaryKey = this.primaryKey;

    return respond((headers, body) => {
      // If updates are true use the info we already have
      // TODO: We may aswell avoid re-fingerprinting here if we are just going
      // to reuse data then its already fingerprinted and as the response is
      // true we don't have anything changed so the old fingerprint stays the
      // same as long as nothing in the fingerprint has been edited (the
      // namespace?)
      if (body === true) {
        body = data;
      }
      return this.fingerprint(
        primaryKey,
        slugKey,
        data[DATACENTER_KEY],
        headers[HTTP_HEADERS_NAMESPACE],
        headers[HTTP_HEADERS_PARTITION]
      )(body);
    });
  }

  respondForDeleteRecord(respond, serialized, data) {
    const slugKey = this.slugKey;
    const primaryKey = this.primaryKey;

    return respond((headers, body) => {
      // Deletes only need the primaryKey/uid returning and they need the slug
      // key AND potential namespace in order to create the correct
      // uid/fingerprint
      return {
        [primaryKey]: this.fingerprint(
          primaryKey,
          slugKey,
          data[DATACENTER_KEY],
          headers[HTTP_HEADERS_NAMESPACE],
          headers[HTTP_HEADERS_PARTITION]
        )({
          [slugKey]: data[slugKey],
          [NSPACE_KEY]: data[NSPACE_KEY],
          [PARTITION_KEY]: data[PARTITION_KEY],
        })[primaryKey],
      };
    });
  }

  // this could get confusing if you tried to override say
  // `normalizeQueryResponse`
  // TODO: consider creating a method for each one of the
  // `normalize...Response` family
  normalizeResponse(store, modelClass, payload, id, requestType) {
    const normalizedPayload = this.normalizePayload(payload, id, requestType);
    // put the meta onto the response, here this is ok as JSON-API allows this
    // and our specific data is now in response[primaryModelClass.modelName]
    // so we aren't in danger of overwriting anything (which was the reason
    // for the Symbol-like property earlier) use a method modelled on
    // ember-data methods so we have the opportunity to do this on a per-model
    // level
    const meta = this.normalizeMeta(store, modelClass, normalizedPayload, id, requestType);
    // get distinct consul versions from list and add it as meta
    if (modelClass.modelName === 'node' && requestType === 'query') {
      meta.versions = this.getDistinctConsulVersions(normalizedPayload);
    }
    if (requestType !== 'query') {
      normalizedPayload.meta = meta;
    }
    const res = super.normalizeResponse(
      store,
      modelClass,
      {
        meta: meta,
        [modelClass.modelName]: normalizedPayload,
      },
      id,
      requestType
    );
    // If the result of the super normalizeResponse is undefined its because
    // the JSONSerializer (which REST inherits from) doesn't recognise the
    // requestType, in this case its likely to be an 'action' request rather
    // than a specific 'load me some data' one. Therefore its ok to bypass the
    // store here for the moment we currently use this for self, but it also
    // would affect any custom methods that use a serializer in our custom
    // service/store
    if (typeof res === 'undefined') {
      return payload;
    }
    return res;
  }

  timestamp() {
    return new Date().getTime();
  }

  normalizeMeta(store, modelClass, payload, id, requestType) {
    // Pick the meta/headers back off the payload and cleanup
    const headers = payload[HTTP_HEADERS_SYMBOL] || {};
    delete payload[HTTP_HEADERS_SYMBOL];
    const meta = {
      cacheControl: headers[HTTP_HEADERS_CACHE_CONTROL.toLowerCase()],
      cursor: headers[HTTP_HEADERS_INDEX.toLowerCase()],
      dc: headers[HTTP_HEADERS_DATACENTER.toLowerCase()],
      nspace: headers[HTTP_HEADERS_NAMESPACE.toLowerCase()],
      partition: headers[HTTP_HEADERS_PARTITION.toLowerCase()],
    };
    if (typeof headers['x-range'] !== 'undefined') {
      meta.range = headers['x-range'];
    }
    if (typeof headers['refresh'] !== 'undefined') {
      meta.interval = headers['refresh'] * 1000;
    }
    if (requestType === 'query') {
      meta.date = this.timestamp();
      payload.forEach(function (item) {
        set(item, 'SyncTime', meta.date);
      });
    }
    return meta;
  }

  normalizePayload(payload, id, requestType) {
    return payload;
  }

  // getDistinctConsulVersions will be called only for nodes and query request type
  // the list of versions is to be added as meta to resp, without changing original response structure
  // hence this function is added in application.js
  getDistinctConsulVersions(payload) {
    // create a Set and add version with only major.minor : ex-1.24.6 as 1.24
    let versionSet = new Set();
    payload.forEach(function (item) {
      if (item.Meta && item.Meta['consul-version'] && item.Meta['consul-version'] !== '') {
        const split = item.Meta['consul-version'].split('.');
        versionSet.add(split[0] + '.' + split[1]);
      }
    });

    const versionArray = Array.from(versionSet);

    if (versionArray.length > 0) {
      // Sort the array in descending order using a custom comparison function
      versionArray.sort((a, b) => {
        // Split the versions into arrays of numbers
        const versionA = a.split('.').map((part) => {
          const number = Number(part);
          return isNaN(number) ? 0 : number;
        });
        const versionB = b.split('.').map((part) => {
          const number = Number(part);
          return isNaN(number) ? 0 : number;
        });

        const minLength = Math.min(versionA.length, versionB.length);

        // start with comparing major version num, if equal then compare minor
        for (let i = 0; i < minLength; i++) {
          if (versionA[i] !== versionB[i]) {
            return versionB[i] - versionA[i];
          }
        }
        return versionB.length - versionA.length;
      });
    }

    return versionArray; //sorted array
  }
}
