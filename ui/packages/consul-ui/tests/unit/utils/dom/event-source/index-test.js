import {
  source,
  proxy,
  cache,
  resolve,
  CallableEventSource,
  OpenableEventSource,
  BlockingEventSource,
  StorageEventSource,
} from 'consul-ui/utils/dom/event-source/index';
import { module, test } from 'qunit';

module('Unit | Utility | dom/event source/index', function () {
  // Replace this with your real tests.
  test('it works', function (assert) {
    // All The EventSource
    assert.strictEqual(typeof CallableEventSource, 'function');
    assert.strictEqual(typeof OpenableEventSource, 'function');
    assert.strictEqual(typeof BlockingEventSource, 'function');
    assert.strictEqual(typeof StorageEventSource, 'function');

    // Utils
    assert.strictEqual(typeof source, 'function');
    assert.strictEqual(typeof proxy, 'function');
    assert.strictEqual(typeof cache, 'function');
    assert.strictEqual(typeof resolve, 'function');
  });
});
