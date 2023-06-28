/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';
import { runInDebug } from '@ember/debug';

const typeAssertion = (type, value, withDefault) => {
  return typeof value === type ? value : withDefault;
};
export default class WithCopyableModifier extends Modifier {
  @service('clipboard/os') clipboard;

  hash = null;
  source = null;

  connect([value], _hash) {
    value = typeAssertion('string', value, this.element.innerText);
    const hash = {
      success: (e) => {
        runInDebug((_) => console.info(`with-copyable: Copied \`${value}\``));
        return typeAssertion('function', _hash.success, () => {})(e);
      },
      error: (e) => {
        runInDebug((_) => console.info(`with-copyable: Error copying \`${value}\``));
        return typeAssertion('function', _hash.error, () => {})(e);
      },
    };
    this.source = this.clipboard
      .execute(this.element, {
        text: (_) => value,
        container: this.element,
        ...hash.options,
      })
      .on('success', hash.success)
      .on('error', hash.error);
    this.hash = hash;
  }

  disconnect() {
    if (this.source && this.hash) {
      this.source.off('success', this.hash.success).off('error', this.hash.error);

      this.source.destroy();
      this.hash = null;
      this.source = null;
    }
  }

  // lifecycle hooks
  didReceiveArguments() {
    this.disconnect();
    this.connect(this.args.positional, this.args.named);
  }

  willRemove() {
    this.disconnect();
  }
}
