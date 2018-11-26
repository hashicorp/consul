import { moduleFor, test } from 'ember-qunit';

moduleFor('route:application', 'Unit | Route | application', {
  // Specify the other units that are required for this test.
  needs: [
    'service:repository/dc',
    'service:settings',
    'service:feedback',
    'service:flashMessages',
    'service:logger',
    'service:dom',
  ],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
