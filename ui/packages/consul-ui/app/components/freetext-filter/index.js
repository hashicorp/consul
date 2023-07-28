import Component from '@glimmer/component';
import { action } from '@ember/object';

const ENTER = 13;
export default class FreetextFilter extends Component {
  get placeholder() {
    return this.args.placeholder || 'Search';
  }

  get onsearch() {
    return this.args.onsearch || (() => {});
  }

  @action
  change(e) {
    this.onsearch(e);
  }

  @action
  keydown(e) {
    if (e.keyCode === ENTER) {
      e.preventDefault();
    }
  }
}
