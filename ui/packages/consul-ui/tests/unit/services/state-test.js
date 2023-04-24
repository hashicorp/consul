import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Service | state', function(hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('.state creates a state matchable object', function(assert) {
    const service = this.owner.lookup('service:state');
    const actual = service.state(id => id === 'idle');
    assert.equal(typeof actual, 'object');
    assert.equal(typeof actual.matches, 'function');
  });
  test('.matches performs a match correctly', function(assert) {
    const service = this.owner.lookup('service:state');
    const state = service.state(id => id === 'idle');
    assert.equal(service.matches(state, 'idle'), true);
    assert.equal(service.matches(state, 'loading'), false);
  });
  test('.matches performs a match correctly when passed an array', function(assert) {
    const service = this.owner.lookup('service:state');
    const state = service.state(id => id === 'idle');
    assert.equal(service.matches(state, ['idle']), true);
    assert.equal(service.matches(state, ['loading', 'idle']), true);
    assert.equal(service.matches(state, ['loading', 'deleting']), false);
  });
});
