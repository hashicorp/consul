/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';

const createEventLike = (state) => {
  return {
    target: state.element,
    currentTarget: state.element,
  };
};

export default class DidUpsertModifier extends Modifier {
  modify(element, positional, named) {
    this.element = element;
    const [fn, ...rest] = positional;
    fn(createEventLike(this), rest, named);
  }
}
