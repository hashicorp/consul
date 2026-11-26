/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit port of tests/acceptance/dc/acls/tokens/sorting.feature.

import { module } from 'qunit';

import {
  setupAcceptanceTest,
  nspaceScenario,
  nspaceURL,
  createModels,
  visit,
  page,
  currentURL,
  assertVisibleCount,
  assertPropertiesVertically,
} from 'consul-ui/tests/helpers/acceptance';

module('Acceptance | dc / acls / tokens / sorting', function (hooks) {
  setupAcceptanceTest(hooks);

  nspaceScenario('Scenario: Sorting Tokens', async function (assert, nspace) {
    createModels('dc', 1, 'dc-1');
    createModels('token', 4, [
      {
        AccessorID: '00000000-0000-0000-0000-000000000001',
        CreateTime: '2018-09-15T11:58:09.197Z',
      },
      {
        AccessorID: '00000000-0000-0000-0000-000000000002',
        CreateTime: '2020-09-15T11:58:09.197Z',
      },
      {
        AccessorID: '00000000-0000-0000-0000-000000000003',
        CreateTime: '2007-09-15T11:58:09.197Z',
      },
      {
        AccessorID: '00000000-0000-0000-0000-000000000004',
        CreateTime: '2011-09-15T11:58:09.197Z',
      },
    ]);

    await visit('tokens', { dc: 'dc-1' }, { nspace });
    assert.equal(currentURL(), nspaceURL(nspace, '/dc-1/acls/tokens'));
    assertVisibleCount(assert, page().tokens, 4, 'token models');

    await page().sort.selected();
    await page().sort.options.objectAt(1).button();
    assertPropertiesVertically(
      assert,
      page().tokens,
      'id',
      [
        '00000000-0000-0000-0000-000000000003',
        '00000000-0000-0000-0000-000000000004',
        '00000000-0000-0000-0000-000000000001',
        '00000000-0000-0000-0000-000000000002',
      ],
      'tokens'
    );

    await page().sort.selected();
    await page().sort.options.objectAt(0).button();
    assertPropertiesVertically(
      assert,
      page().tokens,
      'id',
      [
        '00000000-0000-0000-0000-000000000002',
        '00000000-0000-0000-0000-000000000001',
        '00000000-0000-0000-0000-000000000004',
        '00000000-0000-0000-0000-000000000003',
      ],
      'tokens'
    );
  });
});
