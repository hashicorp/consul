/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/deleting.feature

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  createModels,
  respondWith,
  find,
  visit,
  nspaceEndpoint,
  requestMade,
} from 'consul-ui/tests/helpers/acceptance';

const TOKEN_ID = '001fda31-194e-4ff1-a5ec-589abf2cafd0';
const TOKEN_URL = `/v1/acl/token/${TOKEN_ID}?dc=datacenter&ns=@!namespace`;

const assertNotification = (assert, kind) => {
  assert.dom('[data-notification]').hasClass('hds-toast');
  assert.dom('[data-notification]').hasClass(`hds-alert--color-${kind}`);
};

module(
  'Acceptance | deleting: Deleting items with confirmations, success and error notifications',
  function (hooks) {
    setupAcceptanceTest(hooks);

    nspaceScenario(
      'Scenario: Deleting a token model from the tokens listing page',
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');
        createModels('token', 1, { AccessorID: TOKEN_ID });

        await visit('tokens', { dc: 'datacenter' }, { nspace });
        await find('tokens.actions')();
        await find('tokens.delete')();
        await find('tokens.confirmDelete')();

        requestMade(assert, 'DELETE', nspaceEndpoint(TOKEN_URL, nspace));
        assertNotification(assert, 'success');
      }
    );

    nspaceScenario(
      'Scenario: Deleting a token from the token detail page',
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');

        await visit('token', { dc: 'datacenter', token: TOKEN_ID }, { nspace });
        await find('delete')();
        await find('confirmDelete')();

        requestMade(assert, 'DELETE', nspaceEndpoint(TOKEN_URL, nspace));
        assertNotification(assert, 'success');
      }
    );

    nspaceScenario(
      'Scenario: Deleting a token from the token detail page error',
      async function (assert, nspace) {
        createModels('dc', 1, 'datacenter');

        await visit('token', { dc: 'datacenter', token: TOKEN_ID }, { nspace });
        respondWith(TOKEN_URL, { status: 500 });
        await find('delete')();
        await find('confirmDelete')();

        assertNotification(assert, 'critical');
      }
    );
  }
);
