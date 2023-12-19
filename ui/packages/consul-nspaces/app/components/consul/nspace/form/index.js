/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from "@glimmer/component";
import { action } from "@ember/object";

export default class NspaceForm extends Component {
  @action onSubmit(item) {
    const onSubmit = this.args.onsubmit;
    if (onSubmit) return onSubmit(item);
  }

  @action onDelete(item) {
    const { onsubmit, ondelete } = this.args;

    if (ondelete) {
      return ondelete(item);
    } else {
      if (onsubmit) {
        return onsubmit(item);
      }
    }
  }

  @action onCancel(item) {
    const { oncancel, onsubmit } = this.args;

    if (oncancel) {
      return oncancel(item);
    } else {
      if (onsubmit) {
        return onsubmit(item);
      }
    }
  }
}
