import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { schedule } from '@ember/runloop';

export default class DisclosureComponent extends Component {
  @tracked ids = '';

  @action
  add(id) {
    schedule('afterRender', () => {
      this.ids = `${this.ids}${this.ids.length > 0 ? ` ` : ``}${id}`;
    });
  }

  @action
  remove(id) {
    this.ids = this.ids
      .split(' ')
      .filter((item) => item !== id)
      .join(' ');
  }
}
