import ChildSelectorComponent from './child-selector';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

export default ChildSelectorComponent.extend({
  repo: service('repository/role/component'),
  policyRepo: service('repository/policy'),
  name: 'role',
  state: 'role',
  init: function() {
    this._super(...arguments);
    this.policyForm = this.form.form('policy');
  },
  reset: function(e) {
    this._super(...arguments);
    set(
      this,
      'policy',
      this.policyForm
        .setData(get(this, 'policyRepo').create({ Datacenter: get(this, 'dc') }))
        .getData()
    );
  },
  actions: {
    createPolicy: function() {
      set(
        this,
        'policy',
        this.policyForm
          .setData(get(this, 'policyRepo').create({ Datacenter: get(this, 'dc') }))
          .getData()
      );
      set(this, 'state', 'policy');
    },
  },
});
