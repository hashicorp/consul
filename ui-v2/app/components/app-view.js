import Component from '@ember/component';
import SlotsMixin from 'block-slots';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import templatize from 'consul-ui/utils/templatize';
export default Component.extend(SlotsMixin, {
  loading: false,
  authorized: true,
  enabled: true,
  classNames: ['app-view'],
  classNameBindings: ['enabled::disabled', 'authorized::unauthorized'],
  dom: service('dom'),
  didReceiveAttrs: function() {
    // right now only manually added classes are hoisted to <html>
    const $root = get(this, 'dom').root();
    let cls = get(this, 'class') || '';
    if (get(this, 'loading')) {
      cls += ' loading';
    } else {
      $root.classList.remove(...templatize(['loading']));
    }
    if (cls) {
      // its possible for 'layout' templates to change after insert
      // check for these specific layouts and clear them out
      [...$root.classList].forEach(function(item, i) {
        if (templatize(['edit', 'show', 'list']).indexOf(item) !== -1) {
          $root.classList.remove(item);
        }
      });
      $root.classList.add(...templatize(cls.split(' ')));
    }
  },
  didInsertElement: function() {
    this.didReceiveAttrs();
  },
  didDestroyElement: function() {
    const cls = get(this, 'class') + ' loading';
    if (cls) {
      const $root = get(this, 'dom').root();
      $root.classList.remove(...templatize(cls.split(' ')));
    }
  },
});
