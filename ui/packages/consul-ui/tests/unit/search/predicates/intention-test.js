import predicates from 'consul-ui/search/predicates/intention';
import { search as create } from 'consul-ui/services/search';
import { module, test } from 'qunit';

module('Unit | Search | Predicate | intention', function() {
  const search = create(predicates);
  test('items are found by properties', function(assert) {
    const actual = [
      {
        SourceName: 'Hit',
        DestinationName: 'destination',
      },
      {
        SourceName: 'source',
        DestinationName: 'destination',
      },
      {
        SourceName: 'source',
        DestinationName: 'hiT',
      },
    ].filter(search('hit'));
    assert.equal(actual.length, 2);
  });
  test('items are not found', function(assert) {
    const actual = [
      {
        SourceName: 'source',
        DestinationName: 'destination',
      },
    ].filter(search('hit'));
    assert.equal(actual.length, 0);
  });
  test('items are found by *', function(assert) {
    const actual = [
      {
        SourceName: '*',
        DestinationName: 'destination',
      },
      {
        SourceName: 'source',
        DestinationName: '*',
      },
    ].filter(search('*'));
    assert.equal(actual.length, 2);
  });
  test("* items are found by searching anything in 'All Services (*)'", function(assert) {
    ['All Services (*)', 'SerVices', '(*)', '*', 'vIces', 'lL Ser'].forEach(term => {
      const actual = [
        {
          SourceName: '*',
          DestinationName: 'destination',
        },
        {
          SourceName: 'source',
          DestinationName: '*',
        },
      ].filter(search(term));
      assert.equal(actual.length, 2);
    });
  });
});
