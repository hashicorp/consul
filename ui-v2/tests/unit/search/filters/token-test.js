import getFilter from 'consul-ui/search/filters/token';
import { module, test } from 'qunit';

module('Unit | Search | Filter | token');

const filter = getFilter(cb => cb);
test('items are found by properties', function(assert) {
  [
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
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.ok(actual);
  });
});
test('items are not found', function(assert) {
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
      ServiceIdenitities: [{ ServiceName: 'si' }, { ServiceName: 'si-second' }],
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
test('arraylike things can be empty', function(assert) {
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
    },
    {
      AccessorID: 'id',
      Name: 'name',
      Description: 'description',
      Policies: [],
      Roles: [],
      ServiceIdentities: [],
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
