import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/acls/roles/edit', 'Unit | Route | dc/acls/roles/edit', {
  // Specify the other units that are required for this test.
  needs: [
    'service:repository/role',
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
