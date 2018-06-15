import { module } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import leftTrim from 'consul-ui/utils/left-trim';
module('Unit | Utility | left trim');

test('it trims characters from the left hand side', function(assert) {
  [
    {
      args: ['/a/folder/here', '/'],
      expected: 'a/folder/here',
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
      expected: 'a/folder/here/',
    },
    {
      args: [],
      expected: '',
    },
    {
      args: ['/a/folder/here', '/a/folder'],
      expected: '/here',
    },
    {
      args: ['/a/folder/here/', '/a/folder/here'],
      expected: '/',
    },
    {
      args: ['/a/folder/here/', '/a/folder/here/'],
      expected: '',
    },
  ].forEach(function(item) {
    const actual = leftTrim(...item.args);
    assert.equal(actual, item.expected);
  });
});
