import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import Route from 'consul-ui/routes/dc/acls/tokens/index';

import Mixin from 'consul-ui/mixins/token/with-actions';

module('Unit | Mixin | token/with actions', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.subject = function() {
      const MixedIn = Route.extend(Mixin);
      this.owner.register('test-container:token/with-actions-object', MixedIn);
      return this.owner.lookup('test-container:token/with-actions-object');
    };
  });

  // Replace this with your real tests.
  test('it works', function(assert) {
    const subject = this.subject();
    assert.ok(subject);
  });
});
