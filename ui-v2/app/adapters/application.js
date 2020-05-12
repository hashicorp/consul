import Adapter from './http';
import { inject as service } from '@ember/service';
import { env } from 'consul-ui/env';

export const DATACENTER_QUERY_PARAM = 'dc';
export const NSPACE_QUERY_PARAM = 'ns';
export default Adapter.extend({
  repo: service('settings'),
  client: service('client/http'),
  formatNspace: function(nspace) {
    if (env('CONSUL_NSPACES_ENABLED')) {
      return nspace !== '' ? { [NSPACE_QUERY_PARAM]: nspace } : undefined;
    }
  },
  formatDatacenter: function(dc) {
    return {
      [DATACENTER_QUERY_PARAM]: dc,
    };
  },
  // TODO: kinda protected for the moment
  // decide where this should go either read/write from http
  // should somehow use this or vice versa
  request: function(req, resp, obj, modelName) {
    const client = this.client;
    const store = this.store;
    const adapter = this;

    let unserialized, serialized;
    const serializer = store.serializerFor(modelName);
    // workable way to decide whether this is a snapshot
    // essentially 'is attributable'.
    // Snapshot is private so we can't do instanceof here
    // and using obj.constructor.name gets changed/minified
    // during compilation so you can't rely on it
    // checking for `attributes` being a function is more
    // reliable as that is the thing we need to call
    if (typeof obj.attributes === 'function') {
      unserialized = obj.attributes();
      serialized = serializer.serialize(obj, {});
    } else {
      unserialized = obj;
      serialized = unserialized;
    }

    return client
      .request(function(request) {
        return req(adapter, request, serialized, unserialized);
      })
      .catch(function(e) {
        return adapter.error(e);
      })
      .then(function(respond) {
        // TODO: When HTTPAdapter:responder changes, this will also need to change
        return resp(serializer, respond, serialized, unserialized);
      });
    // TODO: Potentially add specific serializer errors here
    // .catch(function(e) {
    //   return Promise.reject(e);
    // });
  },
});
