import { get } from 'consul-ui/tests/helpers/api';
import { get as _get } from '@ember/object';
import measure from 'consul-ui/tests/helpers/measure';

/* Stub an ember-data adapter response using the private method
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
const stubAdapterResponse = function(cb, payload, adapter) {
  const ajax = adapter._ajaxRequest;
  adapter._ajaxRequest = function(options) {
    options.success(payload, '200', {
      status: 200,
      textStatus: '200',
      getAllResponseHeaders: function() {
        return '';
      },
    });
  };
  return cb().then(function(result) {
    adapter._ajaxRequest = ajax;
    return result;
  });
};
/* `repo` a helper function to faciliate easy integration testing of ember-data Service 'repo' layers
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
 * @param {function} assert  - Your assertion. This receives the result of the preious function as the first
 *                             argument and a function to that receives the stubbed payload giving you an
 *                             opportunity to mutate it before returning for use in your assertion
 */
export default function(name, method, service, stub, test, assert) {
  const adapter = _get(service, 'store').adapterFor(name.toLowerCase());
  let tags = {};
  return stub(function(url, cookies = {}) {
    const key = Object.keys(cookies).filter(function(item) {
      return item.indexOf('COUNT') !== -1;
    });
    tags = {
      count: key.length > 0 ? parseInt(cookies[key[0]]) : 1,
    };
    return get(url, {
      headers: {
        cookie: cookies,
      },
    });
  }).then(function(payload) {
    return stubAdapterResponse(
      function() {
        return measure(
          function() {
            return test(service);
          },
          `${name}Service.${method}`,
          tags
        ).then(function(res) {
          let actual;
          if (typeof res.toArray === 'function') {
            actual = res.toArray().map(function(item) {
              return item.get('data');
            });
          } else {
            if (typeof res.get === 'function') {
              actual = res.get('data');
            } else {
              actual = res;
            }
          }
          assert(actual, function(cb) {
            return cb(payload);
          });
        });
      },
      JSON.parse(JSON.stringify(payload)),
      adapter
    );
  });
}
