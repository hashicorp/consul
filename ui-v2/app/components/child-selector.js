import Component from '@ember/component';
import SlotsMixin from 'block-slots';
import { get, set, computed } from '@ember/object';
import { inject as service } from '@ember/service';
import WithListeners from 'consul-ui/mixins/with-listeners';
import { Promise } from 'rsvp';

export default Component.extend(SlotsMixin, WithListeners, {
  dom: service('dom'),
  builder: service('form'),
  searchBuilder: service('search'),
  onchange: function() {},
  init: function() {
    this._super(...arguments);
    this.form = get(this, 'builder').form(get(this, 'name'));
    this.searchable = get(this, 'searchBuilder').searchable(get(this, 'name'));
  },
  reset: function(e) {
    // TODO: I should be able to reset the ember-data object
    // back to it original state?
    // possibly Forms could know how to create
    set(
      this,
      'item',
      this.form.setData(get(this, 'repo').create({ Datacenter: get(this, 'dc') })).getData()
    );
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
    remove: function(item, items) {
      const prop = get(this, 'repo').getSlugKey();
      const value = get(item, prop);
      const pos = items.findIndex(function(item) {
        return get(item, prop) === value;
      });
      if (pos !== -1) {
        return items.removeAt(pos, 1);
      }
    },
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
    add: function(items, item) {
      set(item, 'CreateTime', new Date().getTime());
      items.pushObject(item);
      // TODO: Fire a proper event
      this.onchange({});
    },
    save: function(item, items, success = function() {}) {
      const repo = get(this, 'repo');
      // It looks like success is the only potentially unsafe
      // operation here
      set(item, 'CreateTime', new Date().getTime());
      // set(this, 'item', repo.persist(item));
      // TODO: temporary async
      // need to be sure that its saved before adding/closing the modal for now
      this.listen(repo.persist(item), 'message', function(e) {
        // TODO: Looks like ember-data doesn't like nested
        // proxy objects
        items.pushObject(e.data);
        success();
      });
    },
    change: function(e, value, item) {
      // TODO: This should potentially be on a onchange handler and dealt
      // with in the Controller using the form there
      const event = get(this, 'dom').normalizeEvent(e, value);
      const form = get(this, 'form');
      try {
        form.handleEvent(event);
      } catch (err) {
        throw err;
      }
    },
  },
});
