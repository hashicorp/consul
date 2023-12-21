import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class Element extends Component {
  @tracked el;

  @tracked touched = false;

  get type() {
    if (typeof this.el !== 'undefined') {
      return this.el.dataset.type || this.el.getAttribute('type') || this.el.getAttribute('role');
    }
    return this.args.type;
  }
  get name() {
    if (typeof this.args.group !== 'undefined') {
      return `${this.args.group.name}[${this.args.name}]`;
    } else {
      return this.args.name;
    }
  }
  get prop() {
    return `${this.args.name.toLowerCase().split('.').join('-')}`;
  }
  get state() {
    const error = this.touched && this.args.error;
    return {
      matches: (name) => name === 'error' && error,
    };
  }

  @action
  connect($el) {
    this.el = $el;
  }
}
