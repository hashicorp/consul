import ChildSelectorComponent from './child-selector';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { alias } from '@ember/object/computed';

import { CallableEventSource as EventSource } from 'consul-ui/utils/dom/event-source';

export default ChildSelectorComponent.extend({
  repo: service('repository/role/component'),
  dom: service('dom'),
  name: 'role',
  type: 'role',
  classNames: ['role-selector'],
  state: 'role',
  // You have to alias data.
  // If you just set it, it loses its reference?
  policy: alias('policyForm.data'),
  init: function() {
    this._super(...arguments);
    this.policyForm = get(this, 'formContainer').form('policy');
    this.source = new EventSource();
  },
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
      const target = event.target;
      switch (target.name) {
        case 'role[state]':
          set(this, 'state', target.value);
          if (target.value === 'policy') {
            get(this, 'dom')
              .component('.code-editor', target.nextElementSibling)
              .didAppear();
          }
          break;
        default:
          this._super(...arguments);
      }
    },
  },
});
