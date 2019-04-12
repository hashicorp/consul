import FormComponent from './form-component';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

export default FormComponent.extend({
  repo: service('repository/policy/component'),
  datacenterRepo: service('repository/dc/component'),
  name: 'policy',
  isScoped: false,
  type: 'policy',
  init: function() {
    this._super(...arguments);
    set(this, 'isScoped', get(this, 'item.Datacenters.length') > 0);
    set(this, 'datacenters', get(this, 'datacenterRepo').findAll());
  },
  actions: {
    change: function() {
      try {
        this._super(...arguments);
      } catch (err) {
        const scoped = get(this, 'isScoped');
        const name = err.target.name;
        const value = err.target.value;
        switch (name) {
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
          case 'policy[type]':
            set(this, 'type', value);
            break;
          default:
            this.onerror(err);
        }
        this.onchange({ target: get(this, 'form') });
      }
    },
  },
});
