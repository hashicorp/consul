import maybeCall from 'consul-ui/utils/maybe-call';
import { module, test } from 'qunit';
import { Promise } from 'rsvp';

module('Unit | Utility | maybe-call', function() {
  test('it calls a function when the resolved value is true', function(assert) {
    assert.expect(1);
    return maybeCall(() => {
      assert.ok(true);
    }, Promise.resolve(true))();
  });
  test("it doesn't call a function when the resolved value is false", function(assert) {
    assert.expect(0);
    return maybeCall(() => {
      assert.ok(true);
    }, Promise.resolve(false))();
  });
});
