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
    $html.classList.add(...templatize(get(this, 'class').split(' ')));
  },
  didDestroyElement: function() {
    $html.classList.remove(...templatize(get(this, 'class').split(' ')));
  },
});
