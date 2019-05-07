import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/acls/tokens/create', 'Unit | Route | dc/acls/tokens/create', {
  // Specify the other units that are required for this test.
  needs: [
    'service:repository/token',
    'service:repository/policy',
    'service:repository/role',
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
