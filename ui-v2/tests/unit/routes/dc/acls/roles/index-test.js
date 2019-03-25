import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/acls/roles/index', 'Unit | Route | dc/acls/roles/index', {
  // Specify the other units that are required for this test.
  needs: [
    'service:repository/role',
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
