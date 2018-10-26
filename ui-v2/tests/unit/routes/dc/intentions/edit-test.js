import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/intentions/edit', 'Unit | Route | dc/intentions/edit', {
  // Specify the other units that are required for this test.
  needs: [
    'service:repository/service',
    'service:repository/intention',
    'service:feedback',
    'service:logger',
    'service:flashMessages',
  ],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
