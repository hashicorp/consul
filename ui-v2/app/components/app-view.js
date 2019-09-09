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
    this._super(...arguments);
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
      const receivedClasses = new Set(templatize(cls.split(' ')));
      const difference = new Set([...$root.classList].filter(item => !receivedClasses.has(item)));
      [...difference].forEach(function(item, i) {
        if (templatize(['edit', 'show', 'list']).indexOf(item) !== -1) {
          $root.classList.remove(item);
        }
      });
      $root.classList.add(...receivedClasses);
    }
  },
  didInsertElement: function() {
    this._super(...arguments);
    this.didReceiveAttrs();
  },
  didDestroyElement: function() {
    this._super(...arguments);
    const cls = get(this, 'class') + ' loading';
    if (cls) {
      const $root = get(this, 'dom').root();
      $root.classList.remove(...templatize(cls.split(' ')));
    }
  },
});
