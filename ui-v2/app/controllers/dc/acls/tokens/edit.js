import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import Changeset from 'ember-changeset';
import validations from 'consul-ui/validations/token';
import lookupValidator from 'ember-changeset-validations';
const normalizeEmberTarget = function(e, value, target) {
  return e.target || { ...target, ...{ name: e, value: value } };
};
export default Controller.extend({
  dom: service('dom'),
  builder: service('form'),
  isScoped: false,
  setProperties: function(model) {
    this.changeset = new Changeset(model.item, lookupValidator(validations), validations);
    const builder = get(this, 'builder').build;
    this.form = builder()
      .setData(this.changeset)
      .add(
        // TODO: Eventually set forms up elsewhere
        builder('policy', {
          Datacenters: {
            type: 'array',
          },
        }).setData(get(model, 'policy'))
      );
    this._super({
      ...model,
      ...{
        item: this.changeset,
      },
    });
  },
  actions: {
    sendClearPolicy: function(item) {
      set(this, 'isScoped', false);
      this.send('clearPolicy', item);
    },
    refreshCodeEditor: function() {
      get(this, 'dom')
        .component('#policy_rules')
        .didAppear();
    },
    change: function(e, value, _target) {
      const form = get(this, 'form');
      try {
        form.handleEvent({ target: normalizeEmberTarget(e, value, _target) });
      } catch (e) {
        switch (e.target.name) {
          case 'policy[isScoped]':
            set(this, 'isScoped', !get(this, 'isScoped'));
            set(this, 'policy[Datacenters]', null);
            break;
          case 'Policy':
            this.send('addPolicy', value);
            form.validate();
            break;
          case 'Details':
            // the Details expander toggle
            // only load on opening
            if (e.target.checked) {
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
