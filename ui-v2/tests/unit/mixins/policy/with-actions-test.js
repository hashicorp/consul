import { moduleFor } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { getOwner } from '@ember/application';
import Route from 'consul-ui/routes/dc/acls/policies/index';

import Mixin from 'consul-ui/mixins/policy/with-actions';

moduleFor('mixin:policy/with-actions', 'Unit | Mixin | policy/with actions', {
  // Specify the other units that are required for this test.
  needs: [
    'mixin:with-blocking-actions',
    'service:feedback',
    'service:flashMessages',
    'service:logger',
    'service:settings',
    'service:repository/policy',
  ],
  subject: function() {
    const MixedIn = Route.extend(Mixin);
    this.register('test-container:policy/with-actions-object', MixedIn);
    return getOwner(this).lookup('test-container:policy/with-actions-object');
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
