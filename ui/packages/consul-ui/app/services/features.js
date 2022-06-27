import Service, { inject as service } from '@ember/service';

export default class FeatureService extends Service {
  @service env;

  get features() {
    return this.env.var('features');
  }

  isEnabled(featureName) {
    return !!this.features?.[featureName];
  }
}
