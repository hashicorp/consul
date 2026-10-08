/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import {
  statusFromChecks,
  healthFromChecks,
  statusFromInstances,
} from 'consul-ui/utils/health-status';

const check = (Status) => ({ Status });

module('Unit | Utility | health-status', function () {
  module('statusFromChecks', function () {
    test('it reports the worst status present', function (assert) {
      assert.strictEqual(statusFromChecks([check('passing'), check('critical')]), 'critical');
      assert.strictEqual(statusFromChecks([check('passing'), check('warning')]), 'warning');
      assert.strictEqual(
        statusFromChecks([check('warning'), check('critical')]),
        'critical',
        'critical outranks warning'
      );
      assert.strictEqual(statusFromChecks([check('passing')]), 'passing');
    });

    test('it treats no checks as empty', function (assert) {
      assert.strictEqual(statusFromChecks([]), 'empty');
      assert.strictEqual(statusFromChecks(), 'empty', 'including when called with nothing');
    });
  });

  module('healthFromChecks', function () {
    test('it counts the checks in the displayed status', function (assert) {
      const health = healthFromChecks([
        check('critical'),
        check('critical'),
        check('passing'),
        check('warning'),
      ]);
      assert.deepEqual(health, { status: 'critical', count: 2, total: 4, passing: 1 });
    });

    test('it reports zero passing when everything is failing', function (assert) {
      const health = healthFromChecks([check('critical'), check('warning')]);
      assert.strictEqual(health.passing, 0, 'lets the badge say "All checks failing"');
      assert.strictEqual(health.total, 2);
    });
  });

  module('statusFromInstances', function () {
    const instance = (Status) => ({ Status });

    test('it reports the worst status across instances', function (assert) {
      assert.strictEqual(
        statusFromInstances([instance('passing'), instance('critical'), instance('warning')]),
        'critical'
      );
      assert.strictEqual(
        statusFromInstances([instance('passing'), instance('warning')]),
        'warning'
      );
      assert.strictEqual(
        statusFromInstances([instance('passing'), instance('passing')]),
        'passing'
      );
    });

    test('it treats no instances, and instances with no checks, as empty', function (assert) {
      assert.strictEqual(statusFromInstances([]), 'empty');
      assert.strictEqual(statusFromInstances(), 'empty');
      assert.strictEqual(statusFromInstances([instance('empty'), instance('empty')]), 'empty');
    });

    test('a single passing instance outranks instances with no checks', function (assert) {
      assert.strictEqual(statusFromInstances([instance('empty'), instance('passing')]), 'passing');
    });
  });
});
