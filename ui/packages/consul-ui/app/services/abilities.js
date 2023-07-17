import Service from 'ember-can/services/can';

export default class AbilitiesService extends Service {
  parse(str) {
    // It's nicer to talk about SSO but technically its part of the authMethod
    // ability, we probably only need 'use SSO' but if we need more, reassess
    // the `replace`
    return super.parse(str.replace('use SSO', 'use authMethods').replace('service', 'zervice'));
  }
}
