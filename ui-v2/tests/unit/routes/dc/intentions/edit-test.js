import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/intentions/edit', 'Unit | Route | dc/intentions/edit', {
  // Specify the other units that are required for this test.
  needs: [
    'service:services',
    'service:intentions',
    'service:feedback',
    'service:logger',
    'service:flashMessages',
  ],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
