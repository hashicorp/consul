// import Route from '@ember/routing/route';
import Route from './index';

export default Route.extend({
  templateName: 'dc/kv/index',
  beforeModel: function(params, transition) {
    if (params.key === '/') {
      this.transitionTo('dc.kv.index');
    }
  },
});
