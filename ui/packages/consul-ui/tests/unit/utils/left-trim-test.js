import { module, test } from 'qunit';
import leftTrim from 'consul-ui/utils/left-trim';

module('Unit | Utility | left trim', function () {
  test('it trims characters from the left hand side', function (assert) {
    assert.expect(8);

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
    ].forEach(function (item) {
      const actual = leftTrim(...item.args);
      assert.equal(actual, item.expected);
    });
  });
});
