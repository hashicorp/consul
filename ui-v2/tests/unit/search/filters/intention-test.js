import getFilter from 'consul-ui/search/filters/intention';
import { module, test } from 'qunit';

module('Unit | Search | Filter | intention');

const filter = getFilter(cb => cb);
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
    const actual = filter(item, {
      s: 'hit',
    });
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
    const actual = filter(item, {
      s: '*',
    });
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
    const actual = filter(item, {
      s: '*',
    });
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
      const actual = filter(item, {
        s: term,
      });
      assert.ok(actual);
    });
  });
});
