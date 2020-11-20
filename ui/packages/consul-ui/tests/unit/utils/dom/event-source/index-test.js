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

module('Unit | Utility | dom/event source/index', function() {
  // Replace this with your real tests.
  test('it works', function(assert) {
    // All The EventSource
    assert.ok(typeof CallableEventSource === 'function');
    assert.ok(typeof OpenableEventSource === 'function');
    assert.ok(typeof BlockingEventSource === 'function');
    assert.ok(typeof StorageEventSource === 'function');

    // Utils
    assert.ok(typeof source === 'function');
    assert.ok(typeof proxy === 'function');
    assert.ok(typeof cache === 'function');
    assert.ok(typeof resolve === 'function');
  });
});
