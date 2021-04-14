import IntlService from 'ember-intl/services/intl';
import { inject as service } from '@ember/service';

class I18nService extends IntlService {
  @service('env') env;
  /**
   * Additionally injects selected project level environment variables into the
   * message formatting context for usage within translated texts
   */
  formatMessage(value, formatOptions) {
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

    formatOptions = {
      ...formatOptions,
      ...env,
    };
    return super.formatMessage(value, formatOptions);
  }
}
export default {
  name: 'i18n',
  initialize: function(container) {
    container.register('service:intl', I18nService);
  },
};
