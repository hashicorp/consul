/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import I18nService, { formatOptionsSymbol } from 'consul-ui/services/i18n';
import ucfirst from 'consul-ui/utils/ucfirst';

import faker from 'faker';

// we currently use HTML in translations, so anything 'word-like' with these
// chars won't get translated
const translator = (cb) => (item) => !['<', '>', '='].includes(item) ? cb(item) : item;

export default class DebugI18nService extends I18nService {
  // formatMessage(value, formatOptions) {
  //   const text = super.formatMessage(...arguments);
  //   let locale = this.env.var('CONSUL_INTL_LOCALE');
  //   if (locale) {
  //     switch (this.env.var('CONSUL_INTL_LOCALE')) {
  //       case '-':
  //         return text
  //           .split(' ')
  //           .map(
  //             translator((item) =>
  //               item
  //                 .split('')
  //                 .map((item) => '-')
  //                 .join('')
  //             )
  //           )
  //           .join(' ');
  //       case 'la-fk':
  //         locale = 'en';
  //       // fallsthrough
  //       default:
  //         faker.locale = locale;
  //         return text
  //           .split(' ')
  //           .map(
  //             translator((item) => {
  //               const word = faker.lorem.word();
  //               return item.charAt(0) === item.charAt(0).toUpperCase() ? ucfirst(word) : word;
  //             })
  //           )
  //           .join(' ');
  //     }
  //   }
  //   return text;
  // }

    // Central debug transformer
  applyDebug(text) {
    const override = this.env.var('CONSUL_INTL_LOCALE');
    if (!override || !text) return text;

    if (override === '-') {
      // Hyphen mask
      return text.replace(/\w/g, '-');
    }

    faker.locale = override === 'la-fk' ? 'en-us' : override;

    // Replace words, preserve tags (<...>) and entities (&...;)
    return text.replace(/\b(\w+)\b/g, (word) => {
      // Skip inside HTML tags/entities
      if (/^[A-Za-z0-9]+$/.test(word)) {
        const fake = faker.lorem.word();
        return word[0] === word[0].toUpperCase() ? ucfirst(fake) : fake;
      }
      return word;
    });
  }

  // Keep t override minimal: fallback for legacy routes.* keys then debug transform
  t(key, options, ...rest) {
    let text = super.t(key, options, ...rest);
    if (text === key && key.startsWith('routes.')) {
      const stripped = key.slice(7);
      const alt = super.t(stripped, options, ...rest);
      if (alt !== stripped) text = alt;
    }
    return this.applyDebug(text);
  }

  // Apply debug to direct formatMessage calls (helpers, components)
  formatMessage(value, formatOptions, ...rest) {
    const text = super.formatMessage(value, formatOptions, ...rest);
    return this.applyDebug(text);
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
