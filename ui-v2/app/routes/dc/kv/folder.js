// import Route from '@ember/routing/route';
import Route from './index';

export default Route.extend({
  templateName: 'dc/kv/index',
  beforeModel: function(transition) {
    this._super(...arguments);
    const params = this.paramsFor('dc.kv.folder');
    if (params.key === '/' || params.key == null) {
      return this.transitionTo('dc.kv.index');
    }
  },
});
