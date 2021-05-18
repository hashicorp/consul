import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class RouteComponent extends Component {
  @service('routlet') routlet;

  @tracked model;

  get title() {
    return this.args.title;
  }

  get params() {
    return this.routlet.paramsFor(this.args.name);
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
