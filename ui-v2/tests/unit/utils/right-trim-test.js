import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import rightTrim from 'consul-ui/utils/right-trim';
module('Unit | Utility | right trim');

test('it trims characters from the right hand side', function(assert) {
  [
    {
      args: ['/a/folder/here/', '/'],
      expected: '/a/folder/here',
    },
    {
      args: ['/a/folder/here', ''],
      expected: '/a/folder/here',
    },
    {
      args: ['a/folder/here', '/'],
      expected: 'a/folder/here',
    },
    {
      args: ['a/folder/here/', '/'],
      expected: 'a/folder/here',
    },
    {
      args: [],
      expected: '',
    },
    {
      args: ['/a/folder/here', '/folder/here'],
      expected: '/a',
    },
    {
      args: ['/a/folder/here', 'a/folder/here'],
      expected: '/',
    },
    {
      args: ['/a/folder/here/', '/a/folder/here/'],
      expected: '',
    },
  ].forEach(function(item) {
    const actual = rightTrim(...item.args);
    assert.equal(actual, item.expected);
  });
});
