import { moduleFor, test } from 'ember-qunit';

moduleFor('route:dc/acls/show', 'Unit | Route | dc/acls/show', {
  // Specify the other units that are required for this test.
  needs: ['service:acls'],
});

test('it exists', function(assert) {
  let route = this.subject();
  assert.ok(route);
});
