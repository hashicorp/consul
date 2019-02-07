import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/services/instance', 'Unit | Route | dc/services/instance', {
  // Specify the other units that are required for this test.
  needs: ['service:repository/service', 'service:repository/proxy'],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
