import domEventSourceStorage from 'consul-ui/utils/dom/event-source/storage';
import { module, test } from 'qunit';

module('Unit | Utility | dom/event source/storage', function () {
  // Replace this with your real tests.
  test('it works', function (assert) {
    let result = domEventSourceStorage(function EventTarget() {});
    assert.ok(result);
  });
});
