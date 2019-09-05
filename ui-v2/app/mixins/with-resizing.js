import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { assert } from '@ember/debug';
export default Mixin.create({
  dom: service('dom'),
  resize: function(e) {
    assert('with-resizing.resize needs to be overridden', false);
  },
  init: function() {
    this._super(...arguments);
    this.handler = e => {
      const win = e.target;
      this.resize({
        detail: { width: win.innerWidth, height: win.innerHeight },
      });
    };
  },
  didInsertElement: function() {
    this._super(...arguments);
    this.dom.viewport().addEventListener('resize', this.handler, false);
    this.didAppear();
  },
  didAppear: function() {
    this.handler({ target: this.dom.viewport() });
  },
  willDestroyElement: function() {
    this.dom.viewport().removeEventListener('resize', this.handler, false);
    this._super(...arguments);
  },
});
