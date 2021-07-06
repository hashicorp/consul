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

// we currently use HTML in translations, so anything 'word-like' with these
// chars won't get translated
const translator = cb => item => (!['<', '>', '='].includes(item) ? cb(item) : item);

class DebugI18nService extends I18nService {
  formatMessage(value, formatOptions) {
    const text = super.formatMessage(...arguments);
    let locale = this.env.var('CONSUL_INTL_LOCALE');
    if (locale) {
      switch (this.env.var('CONSUL_INTL_LOCALE')) {
        case '-':
          return text
            .split(' ')
            .map(
              translator(item =>
                item
                  .split('')
                  .map(item => '-')
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
              translator(item => {
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
export default {
  name: 'debug',
  after: 'i18n',
  initialize(application) {
    application.register('route:application', DebugRoute);
    application.register('service:intl', DebugI18nService);
  },
};
