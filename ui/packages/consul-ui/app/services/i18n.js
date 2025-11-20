import IntlService from 'ember-intl/services/intl';
import { inject as service } from '@ember/service';

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
