import { module } from 'qunit';
import { setupTest } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import Service from '@ember/service';
import Route from 'consul-ui/routes/dc/acls/index';

import Mixin from 'consul-ui/mixins/acl/with-actions';

module('Unit | Mixin | acl/with actions', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.subject = function() {
      const MixedIn = Route.extend(Mixin);
      this.owner.register('test-container:acl/with-actions-object', MixedIn);
      return this.owner.lookup('test-container:acl/with-actions-object');
    };
  });

  // Replace this with your real tests.
  test('it works', function(assert) {
    const subject = this.subject();
    assert.ok(subject);
  });
  test('use persists the token', function(assert) {
    assert.expect(2);
    const item = { ID: 'id' };
    const expected = { Namespace: 'default', AccessorID: null, SecretID: item.ID };
    this.owner.register(
      'service:settings',
      Service.extend({
        persist: function(actual) {
          assert.deepEqual(actual.token, expected);
          return Promise.resolve(actual);
        },
      })
    );
    const subject = this.subject();
    return subject.actions.use
      .bind(subject)(item)
      .then(function(actual) {
        assert.deepEqual(actual.token, expected);
      });
  });
  test('clone clones the token and calls afterDelete correctly', function(assert) {
    assert.expect(4);
    this.owner.register(
      'service:feedback',
      Service.extend({
        execute: function(cb, name) {
          assert.equal(name, 'clone');
          return cb();
        },
      })
    );
    const expected = { ID: 'id' };
    this.owner.register(
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
});
