/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import domEventTargetRsvp from 'consul-ui/utils/dom/event-target/rsvp';
import { module, test } from 'qunit';

module('Unit | Utility | dom/event-target/rsvp', function () {
  // Replace this with your real tests.
  test('it has EventTarget methods', function (assert) {
    const result = domEventTargetRsvp;
    assert.strictEqual(typeof result, 'function');
    ['addEventListener', 'removeEventListener', 'dispatchEvent'].forEach(function (item) {
      assert.strictEqual(typeof result.prototype[item], 'function');
    });
  });
});
