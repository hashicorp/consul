import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import Controller from '@ember/controller';
import Mixin from 'consul-ui/mixins/with-searching';

module('Unit | Mixin | with searching', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.subject = function() {
      const MixedIn = Controller.extend(Mixin);
      this.owner.register('test-container:with-searching-object', MixedIn);
      return this.owner.lookup('test-container:with-searching-object');
    };
  });

  // Replace this with your real tests.
  test('it works', function(assert) {
    const subject = this.subject();
    assert.ok(subject);
  });
});
