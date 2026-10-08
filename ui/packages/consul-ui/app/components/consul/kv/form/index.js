/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action, set } from '@ember/object';
import { inject as service } from '@ember/service';
import keyName from 'consul-ui/utils/keyName';

export default class ConsulKvFormComponent extends Component {
  @service('btoa') encoder;

  @tracked json = true;
  @tracked isConfirmingDelete = false;
  // which of the two failure toasts to word, since a save and a delete both
  // land in the writer's single error state
  @tracked isDeleting = false;

  get folder() {
    return keyName(this.args.parent);
  }

  get name() {
    return keyName(this.args.item?.Key);
  }

  @action
  submit(api, e) {
    this.isDeleting = false;
    return api.submit(e);
  }

  @action
  confirmDelete() {
    this.isConfirmingDelete = true;
  }

  @action
  cancelDelete() {
    this.isConfirmingDelete = false;
  }

  @action
  delete(api) {
    this.isConfirmingDelete = false;
    this.isDeleting = true;
    return api.delete();
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
          this.json = !this.json;
          break;
        default:
          throw err;
      }
    }
  }
}
