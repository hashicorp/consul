import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class RouteComponent extends Component {
  @service('routlet') routlet;

  @tracked model;

  @action
  connect() {
    this.routlet.addRoute(this.args.name, this);
  }
}
