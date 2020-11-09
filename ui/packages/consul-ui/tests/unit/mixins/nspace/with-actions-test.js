import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import Route from 'consul-ui/routes/dc/nspaces/index';

import Mixin from 'consul-ui/mixins/nspace/with-actions';

module('Unit | Mixin | nspace/with actions', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.subject = function() {
      const MixedIn = Route.extend(Mixin);
      this.owner.register('test-container:nspace/with-actions-object', MixedIn);
      return this.owner.lookup('test-container:nspace/with-actions-object');
    };
  });

  // Replace this with your real tests.
  test('it works', function(assert) {
    const subject = this.subject();
    assert.ok(subject);
  });
});
