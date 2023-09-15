import Component from '@ember/component';
import { get, set, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';

import { task } from 'ember-concurrency';

import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  onchange: function () {},
  tagName: '',

  error: function () {},
  type: '',

  dom: service('dom'),
  formContainer: service('form'),

  item: alias('form.data'),

  selectedOptions: alias('items'),

  init: function () {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
    set(this, 'form', this.formContainer.form(this.type));
    this.form.clear({ Datacenter: this.dc, Namespace: this.nspace });
  },
  willDestroyElement: function () {
    this._super(...arguments);
    this._listeners.remove();
  },
  options: computed('selectedOptions.[]', 'allOptions.[]', function () {
    // It's not massively important here that we are defaulting `items` and
    // losing reference as its just to figure out the diff
    let options = this.allOptions || [];
    const items = this.selectedOptions || [];
    if (get(items, 'length') > 0) {
      // filter out any items from the available options that have already been
      // selected/added
      // TODO: find a proper ember-data diff
      options = options.filter((item) => !items.findBy('ID', get(item, 'ID')));
    }
    return options;
  }),
  save: task(function* (item, items, success = function () {}) {
    const repo = this.repo;
    try {
      item = yield repo.persist(item);
      this.actions.change.apply(this, [
        {
          target: {
            name: 'items[]',
            value: items,
          },
        },
        items,
        item,
      ]);
      success();
    } catch (e) {
      this.error({ error: e });
    }
  }),
  actions: {
    reset: function () {
      this.form.clear({ Datacenter: this.dc, Namespace: this.nspace, Partition: this.partition });
    },

    remove: function (item, items) {
      const prop = this.repo.getSlugKey();
      const value = get(item, prop);
      const pos = items.findIndex(function (item) {
        return get(item, prop) === value;
      });
      if (pos !== -1) {
        return items.removeAt(pos, 1);
      }
      this.onchange({ target: this });
    },
    change: function (e, value, item) {
      const event = this.dom.normalizeEvent(...arguments);
      const items = value;
      switch (event.target.name) {
        case 'items[]':
          set(item, 'CreateTime', new Date().getTime());
          // this always happens synchronously
          items.pushObject(item);
          // TODO: Fire a proper event
          this.onchange({ target: this });
          break;
        default:
      }
    },
  },
});
