import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { ref } from 'ember-ref-bucket';
import { htmlSafe } from '@ember/template';

export default class DimensionsProvider extends Component {
  @ref('element') element;

  @tracked height;

  get data() {
    const { height, fillRemainingHeightStyle } = this;

    return {
      height,
      fillRemainingHeightStyle,
    };
  }

  get fillRemainingHeightStyle() {
    return htmlSafe(`height: ${this.height}px;`);
  }

  get bottomBoundary() {
    return document.querySelector(this.args.bottomBoundary) || this.footer;
  }

  get footer() {
    return document.querySelector('footer[role="contentinfo"]');
  }

  @action measureDimensions(element) {
    const bb = this.bottomBoundary.getBoundingClientRect();
    const e = element.getBoundingClientRect();
    this.height = bb.top + bb.height - e.top;
  }

  @action handleWindowResize() {
    const { element } = this;

    this.measureDimensions(element);
  }
}
