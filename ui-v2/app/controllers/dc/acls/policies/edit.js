import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
export default Controller.extend({
  builder: service('form'),
  dom: service('dom'),
  isScoped: false,
  init: function() {
    this._super(...arguments);
    this.form = get(this, 'builder').form('policy');
  },
  setProperties: function(model) {
    // essentially this replaces the data with changesets
    this._super(
      Object.keys(model).reduce((prev, key, i) => {
        switch (key) {
          case 'item':
            prev[key] = this.form.setData(prev[key]).getData();
            break;
        }
        return prev;
      }, model)
    );
    set(this, 'isScoped', get(model.item, 'Datacenters.length') > 0);
  },
  actions: {
    change: function(e, value, item) {
      const form = get(this, 'form');
      const event = get(this, 'dom').normalizeEvent(e, value);
      try {
        form.handleEvent(event);
      } catch (err) {
        const target = event.target;
        switch (target.name) {
          case 'policy[isScoped]':
            set(this, 'isScoped', !get(this, 'isScoped'));
            set(this.item, 'Datacenters', null);
            break;
          default:
            throw err;
        }
      }
    },
  },
});
