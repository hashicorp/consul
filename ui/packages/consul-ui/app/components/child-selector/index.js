import Component from '@ember/component';
import { get, set, computed } from '@ember/object';
import { alias, sort } from '@ember/object/computed';
import { inject as service } from '@ember/service';

import { task } from 'ember-concurrency';

import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  onchange: function() {},
  tagName: '',

  error: function() {},
  type: '',

  dom: service('dom'),
  search: service('search'),
  sort: service('sort'),
  formContainer: service('form'),

  item: alias('form.data'),

  selectedOptions: alias('items'),

  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
    this.searchable = this.search.searchable(this.type);
    this.form = this.formContainer.form(this.type);
    this.form.clear({ Datacenter: this.dc, Namespace: this.nspace });
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this._listeners.remove();
  },
  sortedOptions: sort('allOptions.[]', 'comparator'),
  comparator: computed(function() {
    return this.sort.comparator(this.type)();
  }),
  options: computed('selectedOptions.[]', 'sortedOptions.[]', function() {
    // It's not massively important here that we are defaulting `items` and
    // losing reference as its just to figure out the diff
    let options = this.sortedOptions || [];
    const items = this.selectedOptions || [];
    if (get(items, 'length') > 0) {
      // filter out any items from the available options that have already been
      // selected/added
      // TODO: find a proper ember-data diff
      options = options.filter(item => !items.findBy('ID', get(item, 'ID')));
    }
    this.searchable.add(options);
    return options;
  }),
  save: task(function*(item, items, success = function() {}) {
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
    search: function(term) {
      // TODO: make sure we can either search before things are loaded
      // or wait until we are loaded, guess power select take care of that
      return new Promise(resolve => {
        const remove = this._listeners.add(this.searchable, {
          change: e => {
            remove();
            resolve(e.target.data);
          },
        });
        this.searchable.search(term);
      });
    },
    reset: function() {
      this.form.clear({ Datacenter: this.dc, Namespace: this.nspace });
    },

    remove: function(item, items) {
      const prop = this.repo.getSlugKey();
      const value = get(item, prop);
      const pos = items.findIndex(function(item) {
        return get(item, prop) === value;
      });
      if (pos !== -1) {
        return items.removeAt(pos, 1);
      }
      this.onchange({ target: this });
    },
    change: function(e, value, item) {
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
