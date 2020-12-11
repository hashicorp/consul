import predicates from 'consul-ui/search/predicates/node';
import { search as create } from 'consul-ui/services/search';
import { module, test } from 'qunit';

module('Unit | Search | Predicate | node', function() {
  const search = create(predicates);
  test('items are found by name', function(assert) {
    const actual = [
      {
        Node: 'node-HIT',
        Address: '10.0.0.0',
      },
      {
        Node: 'node',
        Address: '10.0.0.0',
      },
    ].filter(search('hit'));
    assert.equal(actual.length, 1);
  });
  test('items are found by IP address', function(assert) {
    const actual = [
      {
        Node: 'node-HIT',
        Address: '10.0.0.0',
      },
    ].filter(search('10'));
    assert.equal(actual.length, 1);
  });
  test('items are not found by name', function(assert) {
    const actual = [
      {
        Node: 'name',
        Address: '10.0.0.0',
      },
    ].filter(search('hit'));
    assert.equal(actual.length, 0);
  });
  test('items are not found by IP address', function(assert) {
    const actual = [
      {
        Node: 'name',
        Address: '10.0.0.0',
      },
    ].filter(search('9'));
    assert.equal(actual.length, 0);
  });
});
