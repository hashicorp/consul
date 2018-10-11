import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
export default Controller.extend({
  dom: service('dom'),
  builder: service('form'),
  isScoped: false,
  init: function() {
    this._super(...arguments);
    this.form = get(this, 'builder').form('token');
  },
  setProperties: function(model) {
    // essentially this replaces the data with changesets
    this._super(
      Object.keys(model).reduce((prev, key, i) => {
        switch (key) {
          case 'item':
            prev[key] = this.form.setData(prev[key]).getData();
            break;
          case 'policy':
            prev[key] = this.form
              .form(key)
              .setData(prev[key])
              .getData();
            break;
        }
        return prev;
      }, model)
    );
  },
  actions: {
    sendClearPolicy: function(item) {
      set(this, 'isScoped', false);
      this.send('clearPolicy', item);
    },
    refreshCodeEditor: function(selector, parent) {
      if (parent.target) {
        parent = undefined;
      }
      get(this, 'dom')
        .component(selector, parent)
        .didAppear();
    },
    change: function(e, value, item) {
      const form = get(this, 'form');
      try {
        form.handleEvent(get(this, 'dom').normalizeEvent(e, value));
      } catch (e) {
        const target = e.target || { name: null };
        switch (target.name) {
          case 'policy[isScoped]':
            set(this, 'isScoped', !get(this, 'isScoped'));
            set(this.policy, 'Datacenters', null);
            break;
          case 'Policy':
            this.send('addPolicy', value);
            break;
          case 'Details':
            // the Details expander toggle
            // only load on opening
            if (e.target.checked) {
              this.send('refreshCodeEditor', '.code-editor', e.target.parentNode);
              this.send('loadPolicy', value);
            }
            break;
          default:
            throw e;
        }
      }
    },
  },
});
