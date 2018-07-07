import Component from '@ember/component';
import SlotsMixin from 'ember-block-slots';
import { get } from '@ember/object';
const $html = document.documentElement;
const templatize = function(arr = []) {
  return arr.map(item => `template-${item}`);
};
export default Component.extend(SlotsMixin, {
  loading: false,
  classNames: ['app-view'],
  didReceiveAttrs: function() {
    let cls = get(this, 'class') || '';
    if (get(this, 'loading')) {
      cls += ' loading';
    } else {
      $html.classList.remove(...templatize(['loading']));
    }
    if (cls) {
      $html.classList.add(...templatize(cls.split(' ')));
    }
  },
  didInsertElement: function() {
    this.didReceiveAttrs();
  },
  didDestroyElement: function() {
    const cls = get(this, 'class') + ' loading';
    if (cls) {
      $html.classList.remove(...templatize(cls.split(' ')));
    }
  },
});
