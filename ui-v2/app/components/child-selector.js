import Component from '@ember/component';
import SlotsMixin from 'block-slots';
import { get, set, computed } from '@ember/object';
import { inject as service } from '@ember/service';
import { Promise } from 'rsvp';
import WithListeners from 'consul-ui/mixins/with-listeners';
import { alias } from '@ember/object/computed';

export default Component.extend(SlotsMixin, WithListeners, {
  onchange: function() {},

  dom: service('dom'),
  container: service('search'),

  item: alias('form.data'),

  init: function() {
    this._super(...arguments);
    this.searchable = get(this, 'container').searchable(get(this, 'name'));
  },
  options: computed('items.[]', 'allOptions.[]', function() {
    // It's not massively important here that we are defaulting `items` and
    // losing reference as its just to figure out the diff
    let options = get(this, 'allOptions') || [];
    const items = get(this, 'items') || [];
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
    open: function() {
      if (!get(this, 'allOptions.closed')) {
        set(this, 'allOptions', get(this, 'repo').findAllByDatacenter(get(this, 'dc')));
      }
    },
    reset: function() {
      const event = get(this, 'dom').normalizeEvent(...arguments);
      set(this, 'form', event.target);
    },
    save: function(item, items, success = function() {}) {
      const repo = get(this, 'repo');
      set(item, 'CreateTime', new Date().getTime());
      // TODO: temporary async
      // this should be `set(this, 'item', repo.persist(item));`
      // need to be sure that its saved before adding/closing the modal for now
      // and we don't open the modal on prop change yet
      this.listen(repo.persist(item), 'message', e => {
        const item = e.data;
        set(item, 'CreateTime', new Date().getTime());
        items.pushObject(item);
        this.onchange({ target: this });
        // It looks like success is the only potentially unsafe
        // operation here
        success();
      });
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
      // const item = event.target.value;
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
