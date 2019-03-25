import { moduleFor } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { getOwner } from '@ember/application';
import Route from 'consul-ui/routes/dc/acls/roles/index';

import Mixin from 'consul-ui/mixins/role/with-actions';

moduleFor('mixin:policy/with-actions', 'Unit | Mixin | role/with actions', {
  // Specify the other units that are required for this test.
  needs: [
    'mixin:with-blocking-actions',
    'service:feedback',
    'service:flashMessages',
    'service:logger',
    'service:settings',
    'service:repository/role',
  ],
  subject: function() {
    const MixedIn = Route.extend(Mixin);
    this.register('test-container:role/with-actions-object', MixedIn);
    return getOwner(this).lookup('test-container:role/with-actions-object');
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
