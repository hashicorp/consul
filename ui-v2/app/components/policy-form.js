import FormComponent from './form-component';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

export default FormComponent.extend({
  datacenterRepo: service('repository/dc/component'),
  name: 'policy',
  isScoped: false,
  reset: function(e) {
    set(this, 'isScoped', get(this, 'item.Datacenters.length') > 0);
    set(this, 'datacenters', get(this, 'datacenterRepo').findAll());
  },
  actions: {
    change: function() {
      try {
        this._super(...arguments);
      } catch (err) {
        const scoped = get(this, 'isScoped');
        switch (err.target.name) {
          case 'policy[isScoped]':
            if (scoped) {
              set(this, 'previousDatacenters', get(this.item, 'Datacenters'));
              set(this.item, 'Datacenters', null);
            } else {
              set(this.item, 'Datacenters', get(this, 'previousDatacenters'));
              set(this, 'previousDatacenters', null);
            }
            set(this, 'isScoped', !scoped);
            break;
          default:
            // TODO: You can't throw in a component
            throw err;
        }
      }
    },
  },
});
