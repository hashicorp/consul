import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import validations from 'consul-ui/validations/policy';
export default Controller.extend({
  builder: service('form'),
  dom: service('dom'),
  isScoped: false,
  setProperties: function(model) {
    set(this, 'isScoped', get(model.item, 'Datacenters.length') > 0);
    const builder = get(this, 'builder').build;
    // TODO: Eventually set forms up elsewhere
    this.form = builder('policy', {
      Datacenters: {
        type: 'array',
      },
    })
      .setValidators(validations)
      .setData(model.item);
    this._super({
      ...model,
      ...{
        item: this.form.getData(),
      },
    });
  },
  actions: {
    change: function(e, value, item) {
      try {
        get(this, 'form').handleEvent(get(this, 'dom').normalizeEvent(e, value));
      } catch (e) {
        const target = e.target || { name: null };
        switch (target.name) {
          case 'policy[isScoped]':
            set(this, 'isScoped', !get(this, 'isScoped'));
            break;
          default:
            throw e;
        }
      }
    },
  },
});
