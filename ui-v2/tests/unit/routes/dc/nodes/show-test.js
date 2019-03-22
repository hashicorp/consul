import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/nodes/show', 'Unit | Route | dc/nodes/show', {
  // Specify the other units that are required for this test.
  needs: [
    'service:repository/node',
    'service:repository/coordinate',
    'service:repository/session',
    'service:feedback',
    'service:logger',
    'service:flashMessages',
  ],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
