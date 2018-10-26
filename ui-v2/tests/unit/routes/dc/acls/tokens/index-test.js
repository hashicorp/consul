import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/acls/tokens/index', 'Unit | Route | dc/acls/tokens/index', {
  // Specify the other units that are required for this test.
  needs: [
    'service:repository/token',
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
