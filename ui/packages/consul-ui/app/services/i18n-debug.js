/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import I18nService, { formatOptionsSymbol } from 'consul-ui/services/i18n';
import ucfirst from 'consul-ui/utils/ucfirst';

import faker from 'faker';

// we currently use HTML in translations, so anything 'word-like' with these
// chars won't get translated
const translator = (cb) => (item) => !['<', '>', '='].includes(item) ? cb(item) : item;

export default class DebugI18nService extends I18nService {
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
