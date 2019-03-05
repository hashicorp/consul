import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';
import { assert } from '@ember/debug';
export default Mixin.create({
  resize: function(e) {
    assert('with-resizing.resize needs to be overridden', false);
  },
  win: window,
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
    get(this, 'win').addEventListener('resize', this.handler, false);
    this.didAppear();
  },
  didAppear: function() {
    this.handler({ target: get(this, 'win') });
  },
  willDestroyElement: function() {
    get(this, 'win').removeEventListener('resize', this.handler, false);
    this._super(...arguments);
  },
});
