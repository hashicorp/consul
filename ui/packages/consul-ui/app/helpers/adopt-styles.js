/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { assert } from '@ember/debug';
import { adoptStyles } from '@lit/reactive-element';

export default class AdoptStylesHelper extends Helper {
  /**
   * Adopt/apply given styles to a `ShadowRoot` using constructable styleSheets if supported
   *
   * @param {[ShadowRoot, (CSSResultGroup | CSSResultGroup[])]} params
   */
  compute([$shadow, styles], hash) {
    assert(
      'adopt-styles can only be used to apply styles to ShadowDOM elements',
      $shadow instanceof ShadowRoot
    );
    if (!Array.isArray(styles)) {
      styles = [styles];
    }
    adoptStyles($shadow, styles);
  }
}
