import { moduleFor, skip } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { getOwner } from '@ember/application';
import Route from '@ember/routing/route';
import Mixin from 'consul-ui/mixins/with-blocking-actions';

moduleFor('mixin:with-blocking-actions', 'Unit | Mixin | with blocking actions', {
  // Specify the other units that are required for this test.
  needs: ['service:feedback', 'service:flashMessages', 'service:logger'],
  subject: function() {
    const MixedIn = Route.extend(Mixin);
    this.register('test-container:with-blocking-actions-object', MixedIn);
    return getOwner(this).lookup('test-container:with-blocking-actions-object');
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
skip('init sets up feedback properly');
test('afterCreate just calls afterUpdate', function(assert) {
  const subject = this.subject();
  const expected = [1, 2, 3, 4];
  const afterUpdate = this.stub(subject, 'afterUpdate').returns(expected);
  const actual = subject.afterCreate(expected);
  assert.deepEqual(actual, expected);
  assert.ok(afterUpdate.calledOnce);
});
test('afterUpdate calls transitionTo without the last part of the current route name', function(assert) {
  const subject = this.subject();
  const expected = 'dc.kv';
  subject.routeName = expected + '.edit';
  const transitionTo = this.stub(subject, 'transitionTo').returnsArg(0);
  const actual = subject.afterUpdate();
  assert.equal(actual, expected);
  assert.ok(transitionTo.calledOnce);
});
test('afterDelete calls transitionTo without the last part of the current route name if the last part is not `index`', function(assert) {
  const subject = this.subject();
  const expected = 'dc.kv';
  subject.routeName = expected + '.edit';
  const transitionTo = this.stub(subject, 'transitionTo').returnsArg(0);
  const actual = subject.afterDelete();
  assert.equal(actual, expected);
  assert.ok(transitionTo.calledOnce);
});
test('afterDelete calls refresh if the last part is `index`', function(assert) {
  const subject = this.subject();
  subject.routeName = 'dc.kv.index';
  const expected = 'refresh';
  const refresh = this.stub(subject, 'refresh').returns(expected);
  const actual = subject.afterDelete();
  assert.equal(actual, expected);
  assert.ok(refresh.calledOnce);
});
test('the error hooks return type', function(assert) {
  const subject = this.subject();
  const expected = 'success';
  ['errorCreate', 'errorUpdate', 'errorDelete'].forEach(function(item) {
    const actual = subject[item](expected, {});
    assert.equal(actual, expected);
  });
});
test('action cancel just calls afterUpdate', function(assert) {
  const subject = this.subject();
  const expected = [1, 2, 3, 4];
  const afterUpdate = this.stub(subject, 'afterUpdate').returns(expected);
  // TODO: unsure as to whether ember testing should actually bind this for you?
  const actual = subject.actions.cancel.bind(subject)(expected);
  assert.deepEqual(actual, expected);
  assert.ok(afterUpdate.calledOnce);
});
