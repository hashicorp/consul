import getFilter from 'consul-ui/search/filters/role';
import { module, test } from 'qunit';

module('Unit | Search | Filter | role');

const filter = getFilter(cb => cb);
test('items are found by properties', function(assert) {
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
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.notOk(actual);
  });
});
