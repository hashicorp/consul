import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import Changeset from 'ember-changeset';
import validations from 'consul-ui/validations/policy';
import lookupValidator from 'ember-changeset-validations';
import { inject as service } from '@ember/service';
const normalizeEmberTarget = function(e, value, target = {}) {
  return e.target || { ...target, ...{ name: e, value: value } };
};
export default Controller.extend({
  builder: service('form'),
  isScoped: false,
  setProperties: function(model) {
    set(this, 'isScoped', get(model.item, 'Datacenters.length') > 0);
    this.changeset = new Changeset(model.item, lookupValidator(validations), validations);
    const builder = get(this, 'builder').build;
    // TODO: Eventually set forms up elsewhere
    this.form = builder('policy', {
      Datacenters: {
        type: 'array',
      },
    }).setData(this.changeset);
    this._super({
      ...model,
      ...{
        item: this.changeset,
      },
    });
  },
  actions: {
    change: function(e, value, _target) {
      try {
        get(this, 'form').handleEvent({ target: normalizeEmberTarget(e, value) });
      } catch (e) {
        switch (e.target.name) {
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
