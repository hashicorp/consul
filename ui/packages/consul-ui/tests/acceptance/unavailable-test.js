/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { currentURL, visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupTestEnv } from 'consul-ui/services/env';

const unavailableHeaderSelector = '[data-test-unavailable-header]';
const unavailableBodySelector = '[data-test-unavailable-body]';

module('Acceptance | unavailable page', function (hooks) {
  setupApplicationTest(hooks);

  test('it redirects to the unavailable page when the v2 catalog is enabled', async function (assert) {
    assert.expect(3);

    setupTestEnv(this.owner, {
      CONSUL_V2_CATALOG_ENABLED: true,
    });

    await visit('/');
    assert.equal(currentURL(), '/unavailable', 'It should redirect to the unavailable page');

    // Expect the warning message to be visible
    assert.dom(unavailableHeaderSelector).hasText('User Interface Unavailable');
    assert.dom(unavailableBodySelector).exists({ count: 1 });
  });

  test('it does not redirect to the unavailable page', async function (assert) {
    assert.expect(3);

    setupTestEnv(this.owner, {
      CONSUL_V2_CATALOG_ENABLED: false,
    });

    await visit('/');
    assert.equal(
      currentURL(),
      '/dc1/services',
      'It should continue to the services page when v2 catalog is disabled'
    );

    // Expect the warning message to be not be visible
    assert.dom(unavailableHeaderSelector).doesNotExist();
    assert.dom(unavailableBodySelector).doesNotExist();
  });

  test('it redirects away from the unavailable page when v2 catalog is not enabled', async function (assert) {
    assert.expect(3);

    setupTestEnv(this.owner, {
      CONSUL_V2_CATALOG_ENABLED: false,
    });

    await visit('/unavailable');
    assert.equal(currentURL(), '/dc1/services', 'It should redirect to the services page');

    // Expect the warning message to be not be visible
    assert.dom(unavailableHeaderSelector).doesNotExist();
    assert.dom(unavailableBodySelector).doesNotExist();
  });
});
