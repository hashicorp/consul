import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import btoa from 'consul-ui/utils/btoa';
module('Unit | Utils | btoa', {});

test('it encodes strings properly', function(assert) {
  [
    {
      test: '',
      expected: '',
    },
    {
      test: '1234',
      expected: 'MTIzNA==',
    },
  ].forEach(function(item) {
    const actual = btoa(item.test);
    assert.equal(actual, item.expected);
  });
});
