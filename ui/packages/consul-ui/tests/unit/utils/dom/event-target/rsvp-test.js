import domEventTargetRsvp from 'consul-ui/utils/dom/event-target/rsvp';
import { module, test } from 'qunit';

module('Unit | Utility | dom/event-target/rsvp', function() {
  // Replace this with your real tests.
  test('it has EventTarget methods', function(assert) {
    const result = domEventTargetRsvp;
    assert.equal(typeof result, 'function');
    ['addEventListener', 'removeEventListener', 'dispatchEvent'].forEach(function(item) {
      assert.equal(typeof result.prototype[item], 'function');
    });
  });
});
