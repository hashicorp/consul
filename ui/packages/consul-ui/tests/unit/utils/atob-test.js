import { module, skip, test } from 'qunit';
import atob from 'consul-ui/utils/atob';

module('Unit | Utils | atob', function () {
  skip('it decodes non-strings properly', function (assert) {
    [
      {
        test: '        ',
        expected: '',
      },
      {
        test: new String(),
        expected: '',
      },
      {
        test: new String('MTIzNA=='),
        expected: '1234',
      },
      {
        test: [],
        expected: '',
      },
      {
        test: ['   '],
        expected: '',
      },
      {
        test: new Array(),
        expected: '',
      },
      {
        test: ['MTIzNA=='],
        expected: '1234',
      },
      {
        test: null,
        expected: '��e',
      },
    ].forEach(function (item) {
      const actual = atob(item.test);
      assert.equal(actual, item.expected);
    });
  });
  test('it decodes strings properly', function (assert) {
    assert.expect(2);
    [
      {
        test: '',
        expected: '',
      },
      {
        test: 'MTIzNA==',
        expected: '1234',
      },
    ].forEach(function (item) {
      const actual = atob(item.test);
      assert.equal(actual, item.expected);
    });
  });
  test('throws when passed the wrong value', function (assert) {
    assert.expect(4);

    [{}, ['MTIz', 'NA=='], new Number(), 'hi'].forEach(function (item) {
      assert.throws(function () {
        atob(item);
      });
    });
  });
});
