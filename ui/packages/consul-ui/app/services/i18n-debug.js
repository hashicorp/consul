/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import I18nService, { formatOptionsSymbol } from 'consul-ui/services/i18n';
import ucfirst from 'consul-ui/utils/ucfirst';
import { scheduleOnce } from '@ember/runloop';

import faker from 'faker';

// we currently use HTML in translations, so anything 'word-like' with these
// chars won't get translated
const translator = (cb) => (item) => !['<', '>', '='].includes(item) ? cb(item) : item;

export default class DebugI18nService extends I18nService {
  // Override addTranslations to defer updates outside of render cycle
  addTranslations(locale, translations) {
    // Schedule translation additions to happen after render to avoid
    // "Assertion Failed: You attempted to update `_intls`" error
    scheduleOnce('afterRender', this, () => {
      super.addTranslations(locale, translations);
    });
  }

  formatMessage(value, formatOptions) {
    const text = super.formatMessage(...arguments);
    let locale = this.env.var('CONSUL_INTL_LOCALE');
    if (locale) {
      switch (this.env.var('CONSUL_INTL_LOCALE')) {
        case '-':
          return text
            .split(' ')
            .map(
              translator((item) =>
                item
                  .split('')
                  .map((item) => '-')
                  .join('')
              )
            )
            .join(' ');
        case 'la-fk':
          locale = 'en';
        // fallsthrough
        default:
          faker.locale = locale;
          return text
            .split(' ')
            .map(
              translator((item) => {
                const word = faker.lorem.word();
                return item.charAt(0) === item.charAt(0).toUpperCase() ? ucfirst(word) : word;
              })
            )
            .join(' ');
      }
    }
    return text;
  }
  [formatOptionsSymbol]() {
    const formatOptions = super[formatOptionsSymbol](...arguments);
    if (this.env.var('CONSUL_INTL_DEBUG')) {
      return Object.fromEntries(
        // skip ember-intl special props like htmlSafe and default
        Object.entries(formatOptions).map(([key, value]) =>
          !['htmlSafe', 'default'].includes(key) ? [key, `{${key}}`] : [key, value]
        )
      );
    }
    return formatOptions;
  }
}
