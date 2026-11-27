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
  submit,
  currentURL,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

module(
  'Acceptance | dc / intentions / permissions / warn: Intention Permission Warn',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: Changing the action of an intention with permissions warns',
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');
        createModels('intention', 1, {
          SourceName: 'web',
          DestinationName: 'db',
          SourceNS: 'default',
          DestinationNS: 'default',
          SourcePartition: 'default',
          DestinationPartition: 'default',
          Action: null,
          Permissions: [{ Action: 'allow', HTTP: { PathExact: '/path' } }],
        });

        await visit('intention', { dc: 'datacenter', intention: 'intention-id' }, { nspace });
        assert.equal(currentURL(), nspaceURL(nspace, '/datacenter/intentions/intention-id'));
        await click('.value-deny');
        await submit();
        assert.ok(page().warning.see, 'shows the warning');
        await page().warning.cancel.click();
        assert.notOk(page().warning.see, 'the warning is closed');
        await submit();
        assert.ok(page().warning.see, 'shows the warning again');
        await page().warning.confirm.click();
        requestMade(
          assert,
          'PUT',
          '/v1/connect/intentions/exact?source=default%2Fdefault%2Fweb&destination=default%2Fdefault%2Fdb&dc=datacenter'
        );
      }
    );
  }
);
