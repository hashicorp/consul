import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import Route from '@ember/routing/route';
import Mixin from 'consul-ui/mixins/intention/with-actions';

module('Unit | Mixin | intention/with actions', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.subject = function() {
      const MixedIn = Route.extend(Mixin);
      this.owner.register('test-container:intention/with-actions-object', MixedIn);
      return this.owner.lookup('test-container:intention/with-actions-object');
    };
  });

  // Replace this with your real tests.
  test('it works', function(assert) {
    const subject = this.subject();
    assert.ok(subject);
  });
  test('errorCreate returns a different status code if a duplicate intention is found', function(assert) {
    const subject = this.subject();
    const expected = 'exists';
    const actual = subject.errorCreate('error', {
      errors: [{ status: '500', detail: 'duplicate intention found:' }],
    });
    assert.equal(actual, expected);
  });
  test('errorCreate returns the same code if there is no error', function(assert) {
    const subject = this.subject();
    const expected = 'error';
    const actual = subject.errorCreate('error', {});
    assert.equal(actual, expected);
  });
});
