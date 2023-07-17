import { module, test } from 'qunit';
import ucfirst from 'consul-ui/utils/ucfirst';

module('Unit | Utils | ucfirst', function () {
  test('it returns the first letter in uppercase', function (assert) {
    assert.expect(4);

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
    ].forEach(function (item) {
      const actual = ucfirst(item.test);
      assert.equal(actual, item.expected);
    });
  });
});
