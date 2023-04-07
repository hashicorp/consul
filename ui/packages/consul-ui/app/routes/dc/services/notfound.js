import Route from 'consul-ui/routing/route';

export default class NotfoundRoute extends Route {
  redirect(model, transition) {
    this.replaceWith('dc.services.instance', model.name, model.node, model.id);
  }
}
