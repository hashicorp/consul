import Route from 'consul-ui/routing/route';

export default Route.extend({
  beforeModel: function() {
    this.transitionTo('dc.services');
  },
});
