/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import steps from '../steps';

// step definitions that are shared between features should be moved to the
// tests/acceptance/steps/steps.js file

export default function (assert) {
  return steps(assert).then('I should find a file', function () {
    assert.ok(true, this.step);
  });
}
