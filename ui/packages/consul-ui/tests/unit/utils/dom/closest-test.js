import domClosest from 'consul-ui/utils/dom/closest';
import test from 'ember-sinon-qunit/test-support/test';
import { module, skip } from 'qunit';

module('Unit | Utility | dom/closest', function() {
  test('it calls Element.closest with the specified selector', function(assert) {
    const el = {
      closest: this.stub().returnsArg(0),
    };
    const expected = 'selector';
    const actual = domClosest(expected, el);
    assert.equal(actual, expected);
    assert.ok(el.closest.calledOnce);
  });
  test("it fails silently/null if calling closest doesn't work/exist", function(assert) {
    const expected = null;
    const actual = domClosest('selector', {});
    assert.equal(actual, expected);
  });
  skip('polyfill closest');
});
