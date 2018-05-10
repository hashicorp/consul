import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import isFolder from 'consul-ui/utils/isFolder';
module('Unit | Utils | isFolder', {});

test('it detects if a string ends in a slash', function(assert) {
  [
    {
      test: 'hello/world',
      expected: false,
    },
    {
      test: 'hello/world/',
      expected: true,
    },
    {
      test: '/hello/world',
      expected: false,
    },
    {
      test: '//',
      expected: true,
    },
  ].forEach(function(item) {
    const actual = isFolder(item.test);
    assert.equal(actual, item.expected);
  });
});
