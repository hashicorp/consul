import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
export default Controller.extend({
  builder: service('form'),
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
});
