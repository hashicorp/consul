import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import Controller from '@ember/controller';
import Mixin from 'consul-ui/mixins/with-listeners';

module('Unit | Mixin | with listeners', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.subject = function() {
      const MixedIn = Controller.extend(Mixin);
      this.owner.register('test-container:with-listeners-object', MixedIn);
      return this.owner.lookup('test-container:with-listeners-object');
    };
  });

  // Replace this with your real tests.
  test('it works', function(assert) {
    const subject = this.subject();
    assert.ok(subject);
  });
});
