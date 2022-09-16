import { get as httpGet } from 'consul-ui/tests/helpers/api';
import { getProperties, set } from '@ember/object';
import measure from 'consul-ui/tests/helpers/measure';

/**
 * A method that returns all attribute names for a given instance of an ember-data
 * model. This is useful for when we want to get all properties of an ember-data
 * model instance via `getProperties` without passing them explicitly.
 *
 * Example:
 *
 * ```js
 * // get all properties of a service instance
 * return getProperties(service, recordAttributes(service))
 * ```
 *
 * @param {*} record - an ember-data model instance
 * @returns {string[]} An array of attribute names for the model instance
 */
function recordAttributes(record) {
  const attributes = record.constructor.attributes;

  return Array.from(attributes.keys());
}

/** Stub an ember-data adapter response using the private method
 *
 * Allows you to easily specify a HTTP response for the Adapter. The stub only works
 * during the 'lifetime' of `cb` and is reset to normal unstubbed functionality afterwards.
 *
 * Please Note: This overwrites a private ember-data Adapter method, please understand
 * the consequences of doing this if you are using it
 *
 * @param {function} cb - The callback, or test case, to run using the stubbed response
 * @param {object} payload - The payload to use as the response
 * @param {DS.Adapter} adapter - An instance of an ember-data Adapter
 */
const stubAdapterResponse = function (cb, payload, adapter) {
  const payloadClone = JSON.parse(JSON.stringify(payload));
  const client = adapter.client;
  set(adapter, 'client', {
    request: function (cb) {
      return cb(function () {
        const params = client.requestParams(...arguments);
        payload.headers['X-Consul-Namespace'] = params.data.ns || 'default';
        payload.headers['X-Consul-Partition'] = params.data.partition || 'default';
        return Promise.resolve(function (cb) {
          return cb(payload.headers, payloadClone.payload);
        });
      });
    },
  });
  return cb(payload.payload).then(function (result) {
    set(adapter, 'client', client);
    return result;
  });
};
/** `repo` a helper function to faciliate easy integration testing of ember-data Service 'repo' layers
 *
 * Test performance is also measured using `consul-ui/tests/helpers/measure` and therefore results
 * can optionally be sent to a centralized metrics collection stack
 *
 * @param {string}   name    - The name of your repo Service (only used for meta purposes)
 * @param {string}   payload - The method you are testing (only used for meta purposes)
 * @param {Service}  service - An instance of an ember-data based repo Service
 * @param {function} stub    - A function that receives a `stub` function allowing you to stub
 *                             an API endpoint with a set of cookies/env vars used by the double
 * @param {function} test    - Your test case. This function receives an instance of the Service provided
 *                             above as a first and only argument, it should return the result of your test
 * @param {function} assert  - Your assertion. This receives the result of the previous function as the first
 *                             argument and a function to that receives the stubbed payload giving you an
 *                             opportunity to mutate it before returning for use in your assertion
 */
export default function (name, method, service, stub, test, assert) {
  const adapter = service.store.adapterFor(name.toLowerCase());
  let tags = {};
  const requestHeaders = function (url, cookies = {}) {
    const key = Object.keys(cookies).find(function (item) {
      return item.indexOf('COUNT') !== -1;
    });
    tags = {
      count: typeof key !== 'undefined' ? parseInt(cookies[key]) : 1,
    };
    return httpGet(url, {
      headers: {
        cookie: cookies,
      },
    }).then(function (payload) {
      return {
        headers: {},
        payload: payload,
      };
    });
  };

  const parseResponse = function (response) {
    let actual;
    // we are dealing with a record-array
    if (typeof response.toArray === 'function') {
      actual = response.toArray().map(function (item) {
        return getProperties(item, recordAttributes(item));
      });
    } else {
      // we are dealing with a single record
      if (typeof response.get === 'function') {
        const data = response.data;

        if (data) {
          // we were dealing with a proxy
          actual = data;
        } else {
          // we are dealing with a model instance we need to iterate attributes
          actual = getProperties(response, recordAttributes(response));
        }
      } else {
        actual = response;
      }
    }
    return actual;
  };
  return stub(requestHeaders).then(function (payload) {
    return stubAdapterResponse(
      function (payload) {
        return measure(
          function () {
            return test(service);
          },
          `${name}Service.${method}`,
          tags
        ).then(function (response) {
          assert(parseResponse(response), function (cb) {
            return cb(payload);
          });
        });
      },
      payload,
      adapter
    );
  });
}
