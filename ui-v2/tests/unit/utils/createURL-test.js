import { module, skip } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import createURL from 'consul-ui/utils/createURL';
module('Unit | Utils | createURL', {});

skip("it isn't isolated enough, mock encodeURIComponent");
test('it passes the values to encode', function(assert) {
  [
    {
      args: [
        ['/v1/url'],
        ['raw', 'values', 'to', 'encode'],
        {
          query: 'to encode',
          ['key with']: ' spaces ',
        },
      ],
      expected: '/v1/url/raw/values/to/encode?query=to%20encode&key%20with=%20spaces%20',
    },
  ].forEach(function(item) {
    const actual = createURL(...item.args);
    assert.equal(actual, item.expected);
  });
});
test('it adds a query string key without an `=` if the query value is `null`', function(assert) {
  [
    {
      args: [
        ['/v1/url'],
        ['raw', 'values', 'to', 'encode'],
        {
          query: null,
        },
      ],
      expected: '/v1/url/raw/values/to/encode?query',
    },
  ].forEach(function(item) {
    const actual = createURL(...item.args);
    assert.equal(actual, item.expected);
  });
});
