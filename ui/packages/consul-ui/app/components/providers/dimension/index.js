import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { ref } from 'ember-ref-bucket';
import { htmlSafe } from '@ember/template';

export default class DimensionsProvider extends Component {
  @service dom;
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
    return this.args.bottomBoundary || this.footer;
  }

  get footer() {
    return document.querySelector('footer[role="contentinfo"]');
  }

  get viewport() {
    return this.dom.viewport();
  }

  @action measureDimensions(element) {
    const { viewport, bottomBoundary } = this;

    const height =
      viewport.innerHeight - (element.getBoundingClientRect().top + bottomBoundary.clientHeight);

    this.height = height;
  }

  @action handleWindowResize() {
    const { element } = this;

    this.measureDimensions(element);
  }
}
