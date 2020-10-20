import { module } from 'qunit';
import test from 'ember-sinon-qunit/test-support/test';
import ucfirst from 'consul-ui/utils/ucfirst';

module('Unit | Utils | ucfirst', function() {
  test('it returns the first letter in uppercase', function(assert) {
    [
      {
        test: 'hello world',
        expected: 'Hello world',
      },
      {
        test: 'hello World',
        expected: 'Hello World',
      },
      {
        test: 'HELLO WORLD',
        expected: 'HELLO WORLD',
      },
      {
        test: 'hELLO WORLD',
        expected: 'HELLO WORLD',
      },
    ].forEach(function(item) {
      const actual = ucfirst(item.test);
      assert.equal(actual, item.expected);
    });
  });
});
