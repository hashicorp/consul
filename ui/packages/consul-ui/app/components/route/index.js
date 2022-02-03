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
    if(this._model) {
      return this._model;
    }
    if (this.args.name) {
      const outlet = this.routlet.outletFor(this.args.name);
      return this.routlet.modelFor(outlet.name);
    }
    return undefined;
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
