/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import ChildSelectorComponent from '../child-selector/index';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';

export default ChildSelectorComponent.extend({
  repo: service('repository/role'),
  dom: service('dom'),
  name: 'role',
  type: 'role',
  classNames: ['role-selector'],
  isCreatingRole: false,
  confirmRemove: function (item, removeFn) {
    set(this, 'roleToRemove', { item, removeFn });
  },
  cancelRemove: function () {
    set(this, 'roleToRemove', null);
  },
  invokeRemove: function () {
    const { item, removeFn } = this.roleToRemove;
    removeFn(item, this.items);
    set(this, 'roleToRemove', null);
  },
  openRoleForm: function (event) {
    event?.preventDefault();
    set(this, 'roleCreateTrigger', event?.currentTarget);
    set(this, 'isCreatingRole', true);
  },
  closeRoleForm: function (event) {
    event?.preventDefault();
    const trigger = this.roleCreateTrigger;
    this.form.clear({
      Datacenter: this.dc,
      Namespace: this.nspace,
      Partition: this.partition,
    });
    set(this, 'isCreatingRole', false);
    set(this, 'roleCreateTrigger', null);
    scheduleOnce('afterRender', trigger, 'focus');
  },
  saveRole: function (items, event) {
    event?.preventDefault();
    this.save.perform(this.item, items, () => this.closeRoleForm());
  },
  focusRoleForm: function (element) {
    element.querySelector('input')?.focus();
  },
});
