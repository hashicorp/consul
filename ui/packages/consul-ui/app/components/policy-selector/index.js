/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ChildSelectorComponent from '../child-selector/index';
import { set } from '@ember/object';
import { inject as service } from '@ember/service';

const ERROR_PARSE_RULES = 'Failed to parse ACL rules';
const ERROR_INVALID_POLICY = 'Invalid service policy';
const ERROR_NAME_EXISTS = 'Invalid Policy: A Policy with Name';

export default ChildSelectorComponent.extend({
  repo: service('repository/policy'),
  name: 'policy',
  type: 'policy',
  allowIdentity: true,
  classNames: ['policy-selector'],
  init: function () {
    this._super(...arguments);
    const source = this.source;
    if (source) {
      this._listeners.add(source, {
        save: (e) => {
          this.save.perform(...e.data);
        },
      });
    }
  },
  reset: function (e) {
    this._super(...arguments);
    set(this, 'isScoped', false);
  },
  refreshCodeEditor: function (e, target) {
    const selector = '.code-editor';
    this.dom.component(selector, target).didAppear();
  },
  error: function (e) {
    const item = this.item;
    const err = e.error;
    if (typeof err.errors !== 'undefined') {
      const error = err.errors[0];
      let prop = 'Rules';
      let message = error.detail;
      switch (true) {
        case message.indexOf(ERROR_PARSE_RULES) === 0:
        case message.indexOf(ERROR_INVALID_POLICY) === 0:
          prop = 'Rules';
          message = error.detail;
          break;
        case message.indexOf(ERROR_NAME_EXISTS) === 0:
          prop = 'Name';
          message = message.substr(ERROR_NAME_EXISTS.indexOf(':') + 1);
          break;
      }
      if (prop) {
        item.addError(prop, message);
      }
    } else {
      // TODO: Conponents can't throw, use onerror
      throw err;
    }
  },
  openModal: function () {
    const { modal } = this;

    if (modal) {
      modal.open();
    }
  },
  actions: {
    open: function (e) {
      this.refreshCodeEditor(e, e.target.parentElement);
    },
  },
});
