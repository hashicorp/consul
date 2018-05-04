import Component from '@ember/component';
import SlotsMixin from 'ember-block-slots';
import { get } from '@ember/object';
const $html = document.documentElement;
const templatize = function(arr = []) {
  return arr.map(item => `template-${item}`);
};
export default Component.extend(SlotsMixin, {
  classNames: ['app-view'],
  didInsertElement: function() {
    const cls = get(this, 'class');
    if (cls) {
      $html.classList.add(...templatize(cls.split(' ')));
    }
  },
  didDestroyElement: function() {
    const cls = get(this, 'class');
    if (cls) {
      $html.classList.remove(...templatize(cls.split(' ')));
    }
  },
});
