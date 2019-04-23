import ChildSelectorComponent from './child-selector';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

import { alias } from '@ember/object/computed';

import { CallableEventSource as EventSource } from 'consul-ui/utils/dom/event-source';

export default ChildSelectorComponent.extend({
  repo: service('repository/role/component'),
  name: 'role',
  type: 'role',
  classNames: ['role-selector'],
  state: 'role',
  init: function() {
    this._super(...arguments);
    this.policyForm = get(this, 'formContainer').form('policy');
    this.source = new EventSource();
  },
  // You have to alias data
  // is you just set it it loses its reference?
  policy: alias('policyForm.data'),
  actions: {
    reset: function(e) {
      this._super(...arguments);
      get(this, 'policyForm').clear({ Datacenter: get(this, 'dc') });
    },
    dispatch: function(type, data) {
      this.source.dispatchEvent({ type: type, data: data });
    },
    change: function() {
      const event = get(this, 'dom').normalizeEvent(...arguments);
      switch (event.target.name) {
        case 'role[state]':
          set(this, 'state', event.target.value);
          break;
        default:
          this._super(...arguments);
      }
    },
  },
});
