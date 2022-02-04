import Helper from '@ember/component/helper';
import { assert } from '@ember/debug';
import { adoptStyles } from '@lit/reactive-element';

export default class AdoptStylesHelper extends Helper {
  /**
   * Adopt/apply given styles to a `ShadowRoot` using constructable styleSheets if supported
   *
   * @param {[ShadowRoot, CSSResultGroup]} params
   */
  compute([$shadow, styles], hash) {
    assert(
      'adopt-styles can only be used to apply styles to ShadowDOM elements',
      $shadow instanceof ShadowRoot
    );
    adoptStyles($shadow, [styles]);
  }
}
