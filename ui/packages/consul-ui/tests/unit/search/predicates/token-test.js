import { module, test } from 'qunit';

import ExactSearch from 'consul-ui/utils/search/exact';
import predicates from 'consul-ui/search/predicates/token';

module('Unit | Search | Predicate | token', function () {
  test('items are found by properties', function (assert) {
    const actual = new ExactSearch(
      [
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          Policies: [],
        },
        {
          AccessorID: 'HIT-id',
          Name: 'name',
          Description: 'description',
          Policies: [],
        },
        {
          AccessorID: 'id',
          Name: 'name-HIT',
          Description: 'description',
          Policies: [],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'desc-HIT-ription',
          Policies: [],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          Policies: [{ Name: 'policy' }, { Name: 'policy-HIT' }],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          Roles: [{ Name: 'role' }, { Name: 'role-HIT' }],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          ServiceIdentities: [
            { ServiceName: 'service-identity' },
            { ServiceName: 'service-identity-HIT' },
          ],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          NodeIdentities: [{ NodeName: 'node-identity' }, { NodeName: 'node-identity-HIT' }],
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 7);
  });
  test('items are not found', function (assert) {
    const actual = new ExactSearch(
      [
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          Policies: [],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          Policies: [{ Name: 'policy' }, { Name: 'policy-second' }],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          Roles: [{ Name: 'role' }, { Name: 'role-second' }],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          ServiceIdentities: [{ ServiceName: 'si' }, { ServiceName: 'si-second' }],
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          NodeIdentities: [{ NodeName: 'si' }, { NodeName: 'si-second' }],
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
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          Policies: null,
          Roles: null,
          ServiceIdentities: null,
          NodeIdentities: null,
        },
        {
          AccessorID: 'id',
          Name: 'name',
          Description: 'description',
          Policies: [],
          Roles: [],
          ServiceIdentities: [],
          NodeIdentities: [],
        },
      ],
      {
        finders: predicates,
      }
    ).search('hit');
    assert.equal(actual.length, 0);
  });
});
