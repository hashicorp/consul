import { moduleFor } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { getOwner } from '@ember/application';
import Route from 'consul-ui/routes/dc/acls/tokens/index';

import Mixin from 'consul-ui/mixins/token/with-actions';

moduleFor('mixin:token/with-actions', 'Unit | Mixin | token/with actions', {
  // Specify the other units that are required for this test.
  needs: [
    'mixin:with-blocking-actions',
    'service:feedback',
    'service:flashMessages',
    'service:logger',
    'service:settings',
    'service:repository/token',
  ],
  subject: function() {
    const MixedIn = Route.extend(Mixin);
    this.register('test-container:token/with-actions-object', MixedIn);
    return getOwner(this).lookup('test-container:token/with-actions-object');
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
