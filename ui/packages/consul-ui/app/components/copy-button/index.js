import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import chart from './chart.xstate';

export default class CopyButton extends Component {
  @service('clipboard/os') clipboard;
  @service('dom') dom;

  constructor() {
    super(...arguments);
    this.chart = chart;
    this.guid = this.dom.guid(this);
    this._listeners = this.dom.listeners();
  }

  @action
  connect() {
    this._listeners.add(this.clipboard.execute(`#${this.guid} button`), {
      success: () => this.dispatch('SUCCESS'),
      error: () => this.dispatch('ERROR'),
    });
  }

  @action
  disconnect() {
    this._listeners.remove();
  }
}
