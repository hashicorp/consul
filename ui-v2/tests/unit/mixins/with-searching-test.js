import { moduleFor } from 'ember-qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { getOwner } from '@ember/application';
import Controller from '@ember/controller';
import Mixin from 'consul-ui/mixins/with-searching';

moduleFor('mixin:with-searching', 'Unit | Mixin | with searching', {
  // Specify the other units that are required for this test.
  needs: ['service:search', 'service:dom'],
  subject: function() {
    const MixedIn = Controller.extend(Mixin);
    this.register('test-container:with-searching-object', MixedIn);
    return getOwner(this).lookup('test-container:with-searching-object');
  },
});

// Replace this with your real tests.
test('it works', function(assert) {
  const subject = this.subject();
  assert.ok(subject);
});
