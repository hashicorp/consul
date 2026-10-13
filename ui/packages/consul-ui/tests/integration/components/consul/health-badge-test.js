/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | consul/health-badge', function (hooks) {
  setupRenderingTest(hooks);

  module('without counts (services and upstreams lists)', function () {
    test('it renders the status on its own', async function (assert) {
      await render(hbs`<Consul::HealthBadge @status="passing" />`);
      assert.dom('.hds-badge').hasText('Passing');

      await render(hbs`<Consul::HealthBadge @status="warning" />`);
      assert.dom('.hds-badge').hasText('Warning');

      await render(hbs`<Consul::HealthBadge @status="critical" />`);
      assert.dom('.hds-badge').hasText('Critical');
    });

    test('it ignores counts unless @showCount is set', async function (assert) {
      await render(
        hbs`<Consul::HealthBadge @status="critical" @count={{2}} @total={{5}} @passing={{3}} />`
      );
      assert.dom('.hds-badge').hasText('Critical');
    });
  });

  module('with counts (service instances list)', function () {
    test('it describes the checks rather than naming the status', async function (assert) {
      await render(
        hbs`<Consul::HealthBadge @status="critical" @count={{2}} @total={{5}} @passing={{3}} @showCount={{true}} />`
      );
      assert.dom('.hds-badge').hasText('2/5 checks failing');

      await render(
        hbs`<Consul::HealthBadge @status="warning" @count={{3}} @total={{6}} @passing={{3}} @showCount={{true}} />`
      );
      assert.dom('.hds-badge').hasText('3/6 checks failing');
    });

    test('it says all checks are passing when none are failing', async function (assert) {
      await render(
        hbs`<Consul::HealthBadge @status="passing" @count={{4}} @total={{4}} @passing={{4}} @showCount={{true}} />`
      );
      assert.dom('.hds-badge').hasText('All checks passing');
    });

    test('it says all checks are failing when none are passing', async function (assert) {
      await render(
        hbs`<Consul::HealthBadge @status="critical" @count={{5}} @total={{5}} @passing={{0}} @showCount={{true}} />`
      );
      assert.dom('.hds-badge').hasText('All checks failing');

      await render(
        hbs`<Consul::HealthBadge @status="warning" @count={{2}} @total={{2}} @passing={{0}} @showCount={{true}} />`
      );
      assert
        .dom('.hds-badge')
        .hasText('All checks failing', 'a warning with nothing passing is still all failing');
    });
  });

  module('empty and unknown states', function () {
    test('it renders "empty" as plain text rather than a badge', async function (assert) {
      await render(hbs`<Consul::HealthBadge @status="empty" @emptyText="No service checks" />`);
      assert.dom('.hds-badge').doesNotExist('there is no health to badge');
      assert.dom('.consul-health-badge--empty').hasText('No service checks');
    });

    test('the empty text is configurable per column', async function (assert) {
      await render(hbs`<Consul::HealthBadge @status="empty" @emptyText="No node checks" />`);
      assert.dom('.consul-health-badge--empty').hasText('No node checks');
    });

    test('it stays plain text when counts are requested', async function (assert) {
      await render(
        hbs`<Consul::HealthBadge @status="empty" @emptyText="No service checks" @total={{0}} @showCount={{true}} />`
      );
      assert.dom('.hds-badge').doesNotExist();
      assert.dom('.consul-health-badge--empty').hasText('No service checks');
    });

    test('it falls back to "unknown" for an unrecognised status', async function (assert) {
      await render(hbs`<Consul::HealthBadge @status="something-else" />`);
      assert.dom('.hds-badge').hasText('Unknown');

      await render(hbs`<Consul::HealthBadge />`);
      assert.dom('.hds-badge').hasText('Unknown', 'including when no status is passed at all');
    });
  });

  test('it uses the badge text as its accessible label', async function (assert) {
    await render(hbs`<Consul::HealthBadge @status="passing" />`);
    assert.dom('.hds-badge').hasAttribute('aria-label', 'Passing');

    await render(
      hbs`<Consul::HealthBadge @status="passing" @tooltip="All health checks passing" />`
    );
    assert
      .dom('.hds-badge')
      .hasAttribute('aria-label', 'Passing', 'the label stays the status, not the tooltip');
  });

  test('it passes splattributes through in both states', async function (assert) {
    await render(hbs`<Consul::HealthBadge @status="passing" data-test-node-health-checks />`);
    assert.dom('[data-test-node-health-checks]').exists();

    await render(hbs`<Consul::HealthBadge @status="empty" data-test-node-health-checks />`);
    assert.dom('[data-test-node-health-checks]').exists('including when rendered as plain text');
  });
});
