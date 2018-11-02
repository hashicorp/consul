import { moduleFor } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { getOwner } from '@ember/application';
import Service from '@ember/service';
import Route from 'consul-ui/routes/dc/acls/index';

import Mixin from 'consul-ui/mixins/acl/with-actions';

moduleFor('mixin:acl/with-actions', 'Unit | Mixin | acl/with actions', {
  // Specify the other units that are required for this test.
  needs: [
    'mixin:with-blocking-actions',
    'service:feedback',
    'service:flashMessages',
    'service:logger',
    'service:settings',
    'service:repository/acl',
  ],
  subject: function() {
    const MixedIn = Route.extend(Mixin);
    this.register('test-container:acl/with-actions-object', MixedIn);
    return getOwner(this).lookup('test-container:acl/with-actions-object');
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
test('use persists the token and calls transitionTo correctly', function(assert) {
  assert.expect(4);
  this.register(
    'service:feedback',
    Service.extend({
      execute: function(cb, name) {
        assert.equal(name, 'use');
        return cb();
      },
    })
  );
  const item = { ID: 'id' };
  const expectedToken = { AccessorID: null, SecretID: item.ID };
  this.register(
    'service:settings',
    Service.extend({
      persist: function(actual) {
        assert.deepEqual(actual.token, expectedToken);
        return Promise.resolve(actual);
      },
    })
  );
  const subject = this.subject();
  const expected = 'dc.services';
  const transitionTo = this.stub(subject, 'transitionTo').returnsArg(0);
  return subject.actions.use
    .bind(subject)(item)
    .then(function(actual) {
      assert.ok(transitionTo.calledOnce);
      assert.equal(actual, expected);
    });
});
test('clone clones the token and calls afterDelete correctly', function(assert) {
  assert.expect(4);
  this.register(
    'service:feedback',
    Service.extend({
      execute: function(cb, name) {
        assert.equal(name, 'clone');
        return cb();
      },
    })
  );
  const expected = { ID: 'id' };
  this.register(
    'service:repository/acl',
    Service.extend({
      clone: function(actual) {
        assert.deepEqual(actual, expected);
        return Promise.resolve(actual);
      },
    })
  );
  const subject = this.subject();
  const afterDelete = this.stub(subject, 'afterDelete').returnsArg(0);
  return subject.actions.clone
    .bind(subject)(expected)
    .then(function(actual) {
      assert.ok(afterDelete.calledOnce);
      assert.equal(actual, expected);
    });
});
