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

  test('it renders the status on its own when no count is requested', async function (assert) {
    await render(hbs`<Consul::HealthBadge @status="passing" />`);
    assert.dom('.hds-badge').hasText('Passing');

    await render(hbs`<Consul::HealthBadge @status="warning" />`);
    assert.dom('.hds-badge').hasText('Warning');

    await render(hbs`<Consul::HealthBadge @status="critical" />`);
    assert.dom('.hds-badge').hasText('Critical');
  });

  test('it renders a count when asked for one', async function (assert) {
    await render(
      hbs`<Consul::HealthBadge @status="critical" @count={{2}} @total={{5}} @showCount={{true}} />`
    );
    assert.dom('.hds-badge').hasText('2/5 Critical');
  });

  test('it ignores counts unless @showCount is set', async function (assert) {
    await render(hbs`<Consul::HealthBadge @status="critical" @count={{2}} @total={{5}} />`);
    assert.dom('.hds-badge').hasText('Critical');
  });

  test('it renders "empty" as a badge rather than bare text', async function (assert) {
    await render(hbs`<Consul::HealthBadge @status="empty" @showCount={{true}} />`);
    assert.dom('.hds-badge').exists('no-checks still renders a badge');
    assert.dom('.hds-badge').hasText('No checks', 'and carries no meaningless count');
  });

  test('it falls back to "unknown" for an unrecognised status', async function (assert) {
    await render(hbs`<Consul::HealthBadge @status="something-else" />`);
    assert.dom('.hds-badge').hasText('Unknown');

    await render(hbs`<Consul::HealthBadge />`);
    assert.dom('.hds-badge').hasText('Unknown', 'including when no status is passed at all');
  });

  test('it uses the badge text as its accessible label, and @tooltip when given', async function (assert) {
    await render(hbs`<Consul::HealthBadge @status="passing" />`);
    assert.dom('.hds-badge').hasAttribute('aria-label', 'Passing');

    await render(hbs`<Consul::HealthBadge @status="passing" @tooltip="3 of 3 checks passing" />`);
    assert
      .dom('.hds-badge')
      .hasAttribute('aria-label', 'Passing', 'the label stays the status, not the tooltip');
  });

  test('it passes splattributes through to the badge', async function (assert) {
    await render(hbs`<Consul::HealthBadge @status="passing" data-test-node-health-checks />`);
    assert.dom('[data-test-node-health-checks]').exists();
  });
});
