import IntlService from 'ember-intl/services/intl';
import { inject as service } from '@ember/service';
import { scheduleOnce } from '@ember/runloop';

export const formatOptionsSymbol = Symbol();
export default class I18nService extends IntlService {
  @service('env') env;
  /**
   * Additionally injects selected project level environment variables into the
   * message formatting context for usage within translated texts
   */

  constructor(...args) {
    super(...args);
    // Ensure locale array exists immediately
    if (!this.locale || this.locale.length === 0) {
      super.setLocale(['en-us']);
    }
  }

  // Override addTranslations to defer updates outside of render cycle
  // This prevents "Assertion Failed: You attempted to update `_intls`" error
  // that occurs when HDS components try to add translations during rendering
  addTranslations(locale, translations) {
    // Schedule translation additions to happen after render
    scheduleOnce('afterRender', this, this._deferredAddTranslations, locale, translations);
  }

  _deferredAddTranslations(locale, translations) {
    super.addTranslations(locale, translations);
  }

  formatMessage(value, formatOptions, ...rest) {
    formatOptions = this[formatOptionsSymbol](formatOptions);
    return super.formatMessage(value, formatOptions, ...rest);
  }
  [formatOptionsSymbol](options) {
    const env = [
      'CONSUL_HOME_URL',
      'CONSUL_REPO_ISSUES_URL',
      'CONSUL_DOCS_URL',
      'CONSUL_DOCS_LEARN_URL',
      'CONSUL_DOCS_API_URL',
      'CONSUL_COPYRIGHT_URL',
    ].reduce((prev, key) => {
      prev[key] = this.env.var(key);
      return prev;
    }, {});
    return {
      ...options,
      ...env,
    };
  }
}
