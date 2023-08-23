import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class ShadowHostComponent extends Component {

  @tracked shadowRoot;

  @action
  attachShadow($element) {
    this.shadowRoot = $element.attachShadow({ mode: 'open' });
  }

}
