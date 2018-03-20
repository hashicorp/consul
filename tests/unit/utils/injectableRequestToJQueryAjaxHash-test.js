import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import Adapter from 'ember-data/adapters/rest';
import injectableRequestToJQueryAjaxHash from 'consul-ui/utils/injectableRequestToJQueryAjaxHash';
module('Unit | Utils | injectableRequestToJQueryAjaxHash', {});
test('it is exactly the same code as RestAdapter', function(assert) {
  // This will fail when using istanbul/ember-cli-code-coverage as it
  // injects further code into `injectableRequestToJQueryAjaxHash` for instrumentation
  // purposes. It 'looks' like this isn't preventable/ignorable
  const expected = Adapter.create()._requestToJQueryAjaxHash.toString();
  const actual = injectableRequestToJQueryAjaxHash({
    stringify: function(obj) {
      return JSON.stringify(obj);
    },
  }).toString();
  assert.equal(actual, expected);
});
