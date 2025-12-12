/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';
import { runInDebug } from '@ember/debug';
import { registerDestructor } from '@ember/destroyable';

const typeAssertion = (type, value, withDefault) => {
  return typeof value === type ? value : withDefault;
};

function cleanup(instance) {
  if (instance && instance?.source && instance?.hash) {
    instance.source?.off('success', instance.hash.success)?.off('error', instance.hash.error);

    instance.source?.destroy();
    instance.hash = null;
    instance.source = null;
  }
}
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

  constructor() {
    super(...arguments);
    registerDestructor(this, cleanup);
  }

  modify(element, value, namedArgs) {
    this.element = element;
    this.disconnect();
    this.connect(value, namedArgs);
  }

  disconnect() {
    cleanup.call(this);
  }
}
