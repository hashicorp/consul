import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import token from 'consul-ui/validations/token';
import policy from 'consul-ui/validations/policy';
export default Controller.extend({
  dom: service('dom'),
  builder: service('form'),
  isScoped: false,
  setProperties: function(model) {
    const builder = get(this, 'builder').build;
    // TODO: Eventually set forms up elsewhere
    const policyForm = builder('policy', {
      Datacenters: {
        type: 'array',
      },
    })
      .setValidators(policy)
      .setData(model.policy);
    this.form = builder()
      .setValidators(token)
      .add(policyForm)
      .setData(model.item);
    this._super({
      ...model,
      ...{
        item: this.form.getData(),
        policy: policyForm.getData(),
      },
    });
  },
  actions: {
    sendClearPolicy: function(item) {
      set(this, 'isScoped', false);
      this.send('clearPolicy', item);
    },
    refreshCodeEditor: function() {
      // TODO: Shouldn't need to assign an id anymore
      // probably need to unwrap the code-editor element
      get(this, 'dom')
        .component('#policy_rules')
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
            set(this, 'policy[Datacenters]', null);
            break;
          case 'Policy':
            this.send('addPolicy', value);
            // form.validate();
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
