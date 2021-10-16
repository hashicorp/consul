import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class RouteComponent extends Component {
  @service('routlet') routlet;
  @service('router') router;

  @tracked _model;

  get params() {
    return this.routlet.paramsFor(this.args.name);
  }

  get model() {
    if(this.args.name) {
      const temp = this.args.name.split('.');
      temp.pop();
      const name = temp.join('.');
      let model = this.routlet.modelFor(name);
      if(Object.keys(model).length === 0) {
        return null;
      }
      return model;
    }
    return null;
  }

  @action
  connect() {
    this.routlet.addRoute(this.args.name, this);
  }

  @action
  disconnect() {
    this.routlet.removeRoute(this.args.name, this);
  }
}
