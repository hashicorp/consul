import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/acls/edit', 'Unit | Route | dc/acls/edit', {
  // Specify the other units that are required for this test.
  needs: [
    'service:acls',
    'service:settings',
    'service:logger',
    'service:feedback',
    'service:flashMessages',
  ],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
