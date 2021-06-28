import ApplicationRoute from '../routes/application';
import { I18nService, formatOptionsSymbol } from './i18n';
import ucfirst from 'consul-ui/utils/ucfirst';

import faker from 'faker';

let isDebugRoute = false;
const routeChange = function(transition) {
  isDebugRoute = transition.to.name.startsWith('docs');
};
const DebugRoute = class extends ApplicationRoute {
  constructor(owner) {
    super(...arguments);
    this.router = owner.lookup('service:router');
    this.router.on('routeWillChange', routeChange);
  }

  renderTemplate() {
    if (isDebugRoute) {
      this.render('debug');
    } else {
      super.renderTemplate(...arguments);
    }
  }

  willDestroy() {
    this.router.off('routeWillChange', routeChange);
    super.willDestroy(...arguments);
  }
};

class DebugI18nService extends I18nService {
  formatMessage(value, formatOptions) {
    const text = super.formatMessage(...arguments);
    switch (this.env.var('CONSUL_INTL_LOCALE')) {
      case 'la-fk':
        return text
          .split(' ')
          .map(item => {
            const word = faker.lorem.word();
            return item.charAt(0) === item.charAt(0).toUpperCase() ? ucfirst(word) : word;
          })
          .join(' ');
      case '-':
        return text
          .split(' ')
          .map(item =>
            item
              .split('')
              .map(item => '-')
              .join('')
          )
          .join(' ');
    }
    return text;
  }
  [formatOptionsSymbol]() {
    const formatOptions = super[formatOptionsSymbol](...arguments);
    if (this.env.var('CONSUL_INTL_DEBUG')) {
      return Object.fromEntries(
        Object.entries(formatOptions).map(([key, value]) => [key, `{${key}}`])
      );
    }
    return formatOptions;
  }
}
export default {
  name: 'debug',
  after: 'i18n',
  initialize(application) {
    application.register('route:application', DebugRoute);
    application.register('service:intl', DebugI18nService);
  },
};
