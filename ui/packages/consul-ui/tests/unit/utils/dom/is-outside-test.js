import domIsOutside from 'consul-ui/utils/dom/is-outside';
import { module, test } from 'qunit';

module('Unit | Utility | dom/is-outside', function () {
  test('it is outside when its in the document but not in the element', function (assert) {
    // is in the document
    const doc = {
      contains: function (el) {
        return true;
      },
    };
    // is NOT in the element
    const el = {
      contains: function (el) {
        return false;
      },
    };
    const target = {};
    const result = domIsOutside(el, target, doc);
    assert.ok(result);
  });
  test('it is not outside when its not in the document', function (assert) {
    // is NOT in the document
    const doc = {
      contains: function (el) {
        return false;
      },
    };
    // is NOT in the element
    const el = {
      contains: function (el) {
        return false;
      },
    };
    const target = {};
    const result = domIsOutside(el, target, doc);
    assert.notOk(result);
  });
  test('it is not outside when its in the document but in the element', function (assert) {
    // is in the document
    const doc = {
      contains: function (el) {
        return true;
      },
    };
    // is in the element
    const el = {
      contains: function (el) {
        return true;
      },
    };
    const target = {};
    const result = domIsOutside(el, target, doc);
    assert.notOk(result);
  });
  test('it is not outside when its in the document but not in the element', function (assert) {
    // is in the document
    const doc = {
      contains: function (el) {
        return true;
      },
    };
    // is NOT in the element
    const el = {
      contains: function (el) {
        return false;
      },
    };
    // is element
    const target = el;
    const result = domIsOutside(el, target, doc);
    assert.notOk(result);
  });
  test('it is not outside when target is nullish', function (assert) {
    // is in the document
    const doc = {
      contains: function (el) {
        return true;
      },
    };
    // is NOT in the element
    const el = {
      contains: function (el) {
        return false;
      },
    };
    //
    let target = null;
    let result = domIsOutside(el, target, doc);
    assert.notOk(result);
    target = undefined;
    result = domIsOutside(el, target, doc);
    assert.notOk(result);
  });
});
