/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module } from 'qunit';
import { click } from '@ember/test-helpers';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / intentions / permissions / create: Intention Permission Create',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: Adding an L7 permission with a header',
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');

        await visit('intention', { dc: 'datacenter' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/intentions/create'));
        assert.equal(document.title, 'Create Intention - Consul');

        // Specifically set L7
        await click('.value-');

        const { permissions } = page();
        await permissions.create.click();
        await permissions.form.Action.option.Deny.click();
        await permissions.form.PathType.click();
        await permissions.form.PathType.option.PrefixedBy.click();
        await permissions.form.Path.fillIn('/path');
        await permissions.form.Headers.form.Name.fillIn('Name');
        await permissions.form.Headers.form.Value.fillIn('/path/name');
        await permissions.form.Headers.form.submit.click();
        assert.equal(
          permissions.form.Headers.list.intentionPermissionHeaders.length,
          1,
          'one header'
        );
        await permissions.form.submit.click();
        assert.equal(permissions.list.intentionPermissions.length, 1, 'one permission');
      }
    );
  }
);
