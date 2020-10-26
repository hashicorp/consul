import getPredicate from 'consul-ui/search/predicates/intention';
import { module, test } from 'qunit';

module('Unit | Search | Predicate | intention', function() {
  const predicate = getPredicate();
  test('items are found by properties', function(assert) {
    [
      {
        SourceName: 'Hit',
        DestinationName: 'destination',
      },
      {
        SourceName: 'source',
        DestinationName: 'hiT',
      },
    ].forEach(function(item) {
      const actual = predicate('hit')(item);
      assert.ok(actual);
    });
  });
  test('items are not found', function(assert) {
    [
      {
        SourceName: 'source',
        DestinationName: 'destination',
      },
    ].forEach(function(item) {
      const actual = predicate('*')(item);
      assert.notOk(actual);
    });
  });
  test('items are found by *', function(assert) {
    [
      {
        SourceName: '*',
        DestinationName: 'destination',
      },
      {
        SourceName: 'source',
        DestinationName: '*',
      },
    ].forEach(function(item) {
      const actual = predicate('*')(item);
      assert.ok(actual);
    });
  });
  test("* items are found by searching anything in 'All Services (*)'", function(assert) {
    [
      {
        SourceName: '*',
        DestinationName: 'destination',
      },
      {
        SourceName: 'source',
        DestinationName: '*',
      },
    ].forEach(function(item) {
      ['All Services (*)', 'SerVices', '(*)', '*', 'vIces', 'lL Ser'].forEach(function(term) {
        const actual = predicate(term)(item);
        assert.ok(actual);
      });
    });
  });
});
