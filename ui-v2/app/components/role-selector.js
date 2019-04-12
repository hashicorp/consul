import ChildSelectorComponent from './child-selector';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

import { alias } from '@ember/object/computed';

export default ChildSelectorComponent.extend({
  repo: service('repository/role/component'),
  name: 'role',
  state: 'role',
  // You have to alias data
  // is you just set it it loses its reference?
  policy: alias('policyForm.data'),
  actions: {
    reset: function(e) {
      const event = get(this, 'dom').normalizeEvent(...arguments);
      if (event.target.name === 'policy') {
        set(this, 'policyForm', event.target);
      } else {
        this._super(...arguments);
      }
    },
  },
});
