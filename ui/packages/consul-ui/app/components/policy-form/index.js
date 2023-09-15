/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import FormComponent from '../form-component/index';
import { get, set } from '@ember/object';

export default FormComponent.extend({
  type: 'policy',
  name: 'policy',
  allowIdentity: true,
  classNames: ['policy-form'],

  isScoped: false,
  init: function () {
    this._super(...arguments);
    set(this, 'isScoped', get(this, 'item.Datacenters.length') > 0);
    this.templates = [
      {
        name: 'Policy',
        template: '',
      },
      {
        name: 'Service Identity',
        template: 'service-identity',
      },
      {
        name: 'Node Identity',
        template: 'node-identity',
      },
    ];
  },
  actions: {
    change: function (e) {
      try {
        this._super(...arguments);
      } catch (err) {
        const scoped = this.isScoped;
        const name = err.target.name;
        switch (name) {
          case 'policy[isScoped]':
            if (scoped) {
              set(this, 'previousDatacenters', get(this.item, 'Datacenters'));
              set(this.item, 'Datacenters', null);
            } else {
              set(this.item, 'Datacenters', this.previousDatacenters);
              set(this, 'previousDatacenters', null);
            }
            set(this, 'isScoped', !scoped);
            break;
          default:
            this.onerror(err);
        }
        this.onchange({ target: this.form });
      }
    },
  },
});
