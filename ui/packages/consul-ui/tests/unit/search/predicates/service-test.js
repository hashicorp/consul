import { search } from 'consul-ui/services/search';
import spec from 'consul-ui/search/predicates/service';
import { module, test } from 'qunit';

module('Unit | Search | Filter | service', function() {
  const predicate = search(spec);
  test('items are found by properties', function(assert) {
    [
      {
        Name: 'name-HIT',
        Tags: [],
      },
      {
        Name: 'name',
        Tags: ['tag', 'tag-withHiT'],
      },
    ].forEach(function(item) {
      const actual = predicate('hit')(item);
      assert.ok(actual);
    });
  });
  test('items are not found', function(assert) {
    [
      {
        Name: 'name',
      },
      {
        Name: 'name',
        Tags: ['one', 'two'],
      },
    ].forEach(function(item) {
      const actual = predicate('hit')(item);
      assert.notOk(actual);
    });
  });
  test('tags can be empty', function(assert) {
    [
      {
        Name: 'name',
      },
      {
        Name: 'name',
        Tags: null,
      },
      {
        Name: 'name',
        Tags: [],
      },
    ].forEach(function(item) {
      const actual = predicate('hit')(item);
      assert.notOk(actual);
    });
  });
});
