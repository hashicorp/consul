import { moduleFor } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { getOwner } from '@ember/application';
import Route from '@ember/routing/route';
import Mixin from 'consul-ui/mixins/intention/with-actions';

moduleFor('mixin:intention/with-actions', 'Unit | Mixin | intention/with actions', {
  // Specify the other units that are required for this test.
  needs: [
    'mixin:with-blocking-actions',
    'service:feedback',
    'service:flashMessages',
    'service:logger',
  ],
  subject: function() {
    const MixedIn = Route.extend(Mixin);
    this.register('test-container:intention/with-actions-object', MixedIn);
    return getOwner(this).lookup('test-container:intention/with-actions-object');
  },
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
