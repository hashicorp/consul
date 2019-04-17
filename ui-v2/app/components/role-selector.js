import ChildSelectorComponent from './child-selector';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

import { alias } from '@ember/object/computed';

export default ChildSelectorComponent.extend({
  repo: service('repository/role/component'),
  name: 'role',
  classNames: ['role-selector'],
  state: 'role',
  init: function() {
    this._super(...arguments);
    this.policyForm = get(this, 'formContainer').form('policy');
  },
  // You have to alias data
  // is you just set it it loses its reference?
  policy: alias('policyForm.data'),
  actions: {
    reset: function(e) {
      this._super(...arguments);
      get(this, 'policyForm').clear({ Datacenter: get(this, 'dc') });
    },
  },
});
