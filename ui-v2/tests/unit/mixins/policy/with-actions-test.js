import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import Route from 'consul-ui/routes/dc/acls/policies/index';

import Mixin from 'consul-ui/mixins/policy/with-actions';

module('Unit | Mixin | policy/with actions', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.subject = function() {
      const MixedIn = Route.extend(Mixin);
      this.owner.register('test-container:policy/with-actions-object', MixedIn);
      return this.owner.lookup('test-container:policy/with-actions-object');
    };
  });

  // Replace this with your real tests.
  test('it works', function(assert) {
    const subject = this.subject();
    assert.ok(subject);
  });
});
