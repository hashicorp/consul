import { moduleFor, skip } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { getOwner } from '@ember/application';
import Route from '@ember/routing/route';
import Mixin from 'consul-ui/mixins/kv/with-actions';

moduleFor('mixin:kv/with-actions', 'Unit | Mixin | kv/with actions', {
  // Specify the other units that are required for this test.
  needs: [
    'mixin:with-blocking-actions',
    'service:feedback',
    'service:flashMessages',
    'service:logger',
  ],
  subject: function() {
    const MixedIn = Route.extend(Mixin);
    this.register('test-container:kv/with-actions-object', MixedIn);
    return getOwner(this).lookup('test-container:kv/with-actions-object');
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
test('afterUpdate calls transitionTo index when the key is a single slash', function(assert) {
  const subject = this.subject();
  const expected = 'dc.kv.index';
  const transitionTo = this.stub(subject, 'transitionTo').returnsArg(0);
  const actual = subject.afterUpdate({}, { Key: '/' });
  assert.equal(actual, expected);
  assert.ok(transitionTo.calledOnce);
});
test('afterUpdate calls transitionTo folder when the key is not a single slash', function(assert) {
  const subject = this.subject();
  const expected = 'dc.kv.folder';
  const transitionTo = this.stub(subject, 'transitionTo').returnsArg(0);
  ['', '/key', 'key/name'].forEach(item => {
    const actual = subject.afterUpdate({}, { Key: item });
    assert.equal(actual, expected);
  });
  assert.ok(transitionTo.calledThrice);
});
test('afterDelete calls refresh folder when the routeName is `folder`', function(assert) {
  const subject = this.subject();
  subject.routeName = 'dc.kv.folder';
  const refresh = this.stub(subject, 'refresh');
  subject.afterDelete({}, {});
  assert.ok(refresh.calledOnce);
});
skip('action invalidateSession test');
