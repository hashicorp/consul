import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
export default Controller.extend({
  builder: service('form'),
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    this.form = get(this, 'builder').form('role');
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
  },
  actions: {
    change: function(e, value, item) {
      const event = get(this, 'dom').normalizeEvent(e, value);
      const form = get(this, 'form');
      try {
        form.handleEvent(event);
      } catch (err) {
        const target = event.target;
        switch (target.name) {
          case 'Policy':
            set(value, 'CreateTime', new Date().getTime());
            get(this, 'item.Policies').pushObject(value);
            break;
          case 'PolicyDetails':
            // the Details expander toggle
            // only load on opening
            if (target.checked) {
              this.send('refreshCodeEditor', '.code-editor', target.parentNode);
              if (!get(value, 'Rules')) {
                this.send('loadPolicy', value, get(this, 'item.Policies'));
              }
            }
            break;
          default:
            throw err;
        }
      }
    },
  },
});
