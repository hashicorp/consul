import { moduleFor, test } from 'ember-qunit';

moduleFor('service:code-mirror/linter', 'Unit | Service | code mirror/linter', {
  // Specify the other units that are required for this test.
  needs: ['service:dom'],
});

// Replace this with your real tests.
test('it exists', function(assert) {
  let service = this.subject();
  assert.ok(service);
});
