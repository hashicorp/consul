import Route from 'consul-ui/routing/route';

export default Route.extend({
  redirect: function(model, transition) {
    this.replaceWith('dc.services.instance', model.name, model.node, model.id);
  },
});
