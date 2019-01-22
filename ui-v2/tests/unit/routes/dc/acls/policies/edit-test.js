import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/acls/policies/edit', 'Unit | Route | dc/acls/policies/edit', {
  // Specify the other units that are required for this test.
  needs: [
    'service:repository/policy',
    'service:repository/token',
    'service:repository/dc',
    'service:feedback',
    'service:logger',
    'service:settings',
    'service:flashMessages',
  ],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
