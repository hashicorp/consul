import Component from '@ember/component';
import SlotsMixin from 'ember-block-slots';
import { get } from '@ember/object';
import templatize from 'consul-ui/utils/templatize';
const $html = document.documentElement;
export default Component.extend(SlotsMixin, {
  loading: false,
  authorized: true,
  enabled: true,
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
