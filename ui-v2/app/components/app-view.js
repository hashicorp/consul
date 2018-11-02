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
  classNameBindings: ['enabled::disabled', 'authorized::unauthorized'],
  didReceiveAttrs: function() {
    // right now only manually added classes are hoisted to <html>
    let cls = get(this, 'class') || '';
    if (get(this, 'loading')) {
      cls += ' loading';
    } else {
      $html.classList.remove(...templatize(['loading']));
    }
    if (cls) {
      // its possible for 'layout' templates to change after insert
      // check for these specific layouts and clear them out
      [...$html.classList].forEach(function(item, i) {
        if (templatize(['edit', 'show', 'list']).indexOf(item) !== -1) {
          $html.classList.remove(item);
        }
      });
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
