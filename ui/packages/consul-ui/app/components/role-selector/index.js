/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ChildSelectorComponent from '../child-selector/index';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';
import { alias } from '@ember/object/computed';

import { CallableEventSource as EventSource } from 'consul-ui/utils/dom/event-source';

export default ChildSelectorComponent.extend({
  repo: service('repository/role'),
  dom: service('dom'),
  name: 'role',
  type: 'role',
  classNames: ['role-selector'],
  state: 'role',
  // You have to alias data.
  // If you just set it, it loses its reference?
  policy: alias('policyForm.data'),
  init: function () {
    this._super(...arguments);
    set(this, 'policyForm', this.formContainer.form('policy'));
    this.source = new EventSource();
  },
  actions: {
    reset: function (e) {
      this._super(...arguments);
      this.policyForm.clear({ Datacenter: this.dc });
    },
    dispatch: function (type, data) {
      this.source.dispatchEvent({ type: type, data: data });
    },
    change: function () {
      const event = this.dom.normalizeEvent(...arguments);
      const target = event.target;
      switch (target.name) {
        case 'role[state]':
          set(this, 'state', target.value);
          if (target.value === 'policy') {
            this.dom.component('.code-editor', target.nextElementSibling).didAppear();
          }
          break;
        default:
          this._super(...arguments);
      }
    },
    triggerStateCheckboxChange() {
      //Triggers click event on checkbox
      //The function has to be added to change the logic from <label for=''> to Hds::Button
      let element = document.getElementById(`${this.name}_state_policy`);
      element && element.dispatchEvent(new Event('change'));
    },
  },
});
