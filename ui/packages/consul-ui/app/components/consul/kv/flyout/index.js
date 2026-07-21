/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action, set } from '@ember/object';
import { inject as service } from '@ember/service';

export default class ConsulKvFlyoutComponent extends Component {
  @service('btoa') encoder;

  formId = 'consul-kv-flyout-form';

  @tracked json = true;
  @tracked session = null;

  @action
  setSession(event) {
    this.session = event.data;
  }

  @action
  change(e, form) {
    const item = form.getData();
    try {
      form.handleEvent(e);
    } catch (err) {
      const target = e.target;
      let parent;
      switch (target.name) {
        case 'value':
          set(item, 'Value', this.encoder.execute(target.value));
          break;
        case 'additional':
          parent = this.args.parent;
          set(item, 'Key', `${parent !== '/' ? parent : ''}${target.value}`);
          break;
        case 'json':
          // TODO: Potentially save whether json has been clicked to the model,
          // setting this.json = true here will force the form to always default to code=on
          // even if the user has selected code=off on another KV
          // ideally we would save the value per KV, but I'd like to not do that on the model
          // a this.json = valueFromSomeStorageJustForThisKV would be added here
          this.json = !this.json;
          break;
        default:
          throw err;
      }
    }
  }
}
