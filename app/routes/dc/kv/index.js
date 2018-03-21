import Route from '@ember/routing/route';

import WithKeyUtils from 'consul-ui/mixins/with-key-utils';
export default Route.extend(WithKeyUtils, {
  beforeModel: function() {
    this.transitionTo('dc.kv.show', this.rootKey);
  },
});
