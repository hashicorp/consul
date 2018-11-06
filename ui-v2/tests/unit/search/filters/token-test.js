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
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.ok(!actual);
  });
});
test('policies can be empty', function(assert) {
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
    },
    {
      AccessorID: 'id',
      Name: 'name',
      Description: 'description',
      Policies: [],
    },
  ].forEach(function(item) {
    const actual = filter(item, {
      s: 'hit',
    });
    assert.ok(!actual);
  });
});
