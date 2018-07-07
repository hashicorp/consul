import { moduleFor, test } from 'ember-qunit';

moduleFor('route:settings', 'Unit | Route | settings', {
  // Specify the other units that are required for this test.
  needs: [
    'service:dc',
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
