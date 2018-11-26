import { module } from 'qunit';
import test from 'ember-sinon-qunit/test-support/test';
import { setupTest } from 'ember-qunit';
import EmberObject from '@ember/object';
import Mixin from 'consul-ui/mixins/with-resizing';
module('Integration | Mixin | with-resizing', function(hooks) {
  setupTest(hooks);
  test('window.addEventListener, resize and window.removeEventListener are called once each through the entire lifecycle', function(assert) {
    const win = {
      innerWidth: 0,
      innerHeight: 0,
      addEventListener: this.stub(),
      removeEventListener: this.stub(),
    };
    const dom = {
      viewport: function() {
        return win;
      },
    };
    const subject = EmberObject.extend(Mixin, {
      dom: dom,
    }).create();
    const resize = this.stub(subject, 'resize');
    subject.didInsertElement();
    subject.willDestroyElement();
    assert.ok(win.addEventListener.calledOnce);
    assert.ok(resize.calledOnce);
    assert.ok(resize.calledWith({ detail: { width: 0, height: 0 } }));
    assert.ok(win.removeEventListener.calledOnce);
  });
});
