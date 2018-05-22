import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/acls/create', 'Unit | Route | dc/acls/create', {
  // Specify the other units that are required for this test.
  needs: [
    'service:acls',
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
