import predicates from 'consul-ui/search/predicates/role';
import { search as create } from 'consul-ui/services/search';
import { module, test } from 'qunit';

module('Unit | Search | Predicate | role', function() {
  const search = create(predicates);
  test('items are found by properties', function(assert) {
    const actual = [
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
    ].filter(search('hit'));
    assert.equal(actual.length, 4);
  });
  test('items are not found', function(assert) {
    const actual = [
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
    ].filter(search('hit'));
    assert.equal(actual.length, 0);
  });
  test('arraylike things can be empty', function(assert) {
    const actual = [
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
    ].filter(search('hit'));
    assert.equal(actual.length, 0);
  });
});
