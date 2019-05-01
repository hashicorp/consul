import Component from '@ember/component';
import { get, set, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { Promise } from 'rsvp';

import SlotsMixin from 'block-slots';
import WithListeners from 'consul-ui/mixins/with-listeners';

export default Component.extend(SlotsMixin, WithListeners, {
  onchange: function() {},

  error: function() {},
  type: '',

  dom: service('dom'),
  container: service('search'),
  formContainer: service('form'),

  item: alias('form.data'),

  selectedOptions: alias('items'),

  init: function() {
    this._super(...arguments);
    this.searchable = get(this, 'container').searchable(get(this, 'type'));
    this.form = get(this, 'formContainer').form(get(this, 'type'));
    this.form.clear({ Datacenter: get(this, 'dc') });
  },
  options: computed('selectedOptions.[]', 'allOptions.[]', function() {
    // It's not massively important here that we are defaulting `items` and
    // losing reference as its just to figure out the diff
    let options = get(this, 'allOptions') || [];
    const items = get(this, 'selectedOptions') || [];
    if (get(items, 'length') > 0) {
      // find a proper ember-data diff
      options = options.filter(item => !items.findBy('ID', get(item, 'ID')));
      this.searchable.add(options);
    }
    return options;
  }),
  actions: {
    search: function(term) {
      // TODO: make sure we can either search before things are loaded
      // or wait until we are loaded, guess power select take care of that
      return new Promise(resolve => {
        const remove = this.listen(this.searchable, 'change', function(e) {
          remove();
          resolve(e.target.data);
        });
        this.searchable.search(term);
      });
    },
    reset: function() {
      get(this, 'form').clear({ Datacenter: get(this, 'dc') });
    },
    open: function() {
      if (!get(this, 'allOptions.closed')) {
        set(this, 'allOptions', get(this, 'repo').findAllByDatacenter(get(this, 'dc')));
      }
    },
    save: function(item, items, success = function() {}) {
      // Specifically this saves an 'new' option/child
      // and then adds it to the selectedOptions, not options
      const repo = get(this, 'repo');
      set(item, 'CreateTime', new Date().getTime());
      // TODO: temporary async
      // this should be `set(this, 'item', repo.persist(item));`
      // need to be sure that its saved before adding/closing the modal for now
      // and we don't open the modal on prop change yet
      item = repo.persist(item);
      this.listen(item, 'message', e => {
        this.actions.change.bind(this)(
          {
            target: {
              name: 'items[]',
              value: items,
            },
          },
          items,
          e.data
        );
        success();
      });
      this.listen(item, 'error', this.error.bind(this));
    },
    remove: function(item, items) {
      const prop = get(this, 'repo').getSlugKey();
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
      const event = get(this, 'dom').normalizeEvent(...arguments);
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
