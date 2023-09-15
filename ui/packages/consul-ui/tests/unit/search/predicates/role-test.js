/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';

import ExactSearch from 'consul-ui/utils/search/exact';
import predicates from 'consul-ui/search/predicates/role';

module('Unit | Search | Predicate | role', function () {
  test('items are found by properties', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Name: 'name-HIT',
          Description: 'description',
          Policies: [],
        },
        {
          Name: 'name',
          Description: 'desc-HIT-ription',
          Policies: [],
        },
        {
          Name: 'name',
          Description: 'description',
          Policies: [{ Name: 'policy' }, { Name: 'policy-HIT' }],
        },
        {
          Name: 'name',
          Description: 'description',
          ServiceIdentities: [
            { ServiceName: 'service-identity' },
            { ServiceName: 'service-identity-HIT' },
          ],
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 4);
  });
  test('items are not found', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Name: 'name',
          Description: 'description',
          Policies: [],
        },
        {
          Name: 'name',
          Description: 'description',
          Policies: [{ Name: 'policy' }, { Name: 'policy-second' }],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          ServiceIdenitities: [{ ServiceName: 'si' }, { ServiceName: 'si-second' }],
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
  test('arraylike things can be empty', function (assert) {
    const actual = new ExactSearch(
      [
        {
          Name: 'name',
          Description: 'description',
        },
        {
          Name: 'name',
          Description: 'description',
          Policies: null,
          ServiceIdentities: null,
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          Policies: [],
          ServiceIdentities: [],
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
});
