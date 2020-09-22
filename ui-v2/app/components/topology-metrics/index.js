import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class TopologyMetrics extends Component {
  // =services
  @service dom;

  // =attributes
  tagName = '';

  @tracked downView;
  @tracked downCardDimensions = [];
  @tracked centerDimensions;
  @tracked upView;
  @tracked upCardDimensions = [];

  // =methods
  getDownCards() {
    return document.querySelectorAll('[id^="downstreamCard"]');
  }

  getGrafana() {
    return document.getElementById('metrics-container');
  }

  getUpCards() {
    return document.querySelectorAll('[id^="upstreamCard"]');
  }

  getDimensions(item) {
    return this.dom.element(item).getBoundingClientRect();
  }

  getSVGDimensions(item) {
    const $el = this.dom.element('#' + item);
    const $refs = [$el.offsetParent, $el];

    return $refs.reduce(
      function(prev, item) {
        prev.x += item.offsetLeft;
        prev.y += item.offsetTop;
        return prev;
      },
      {
        x: 0,
        y: 0,
        height: $el.offsetHeight,
        width: $el.offsetWidth,
      }
    );
  }

  curve() {
    const args = [...arguments];
    return `${arguments.length > 2 ? `C` : `Q`} ${args
      .concat(args.shift())
      .map(p => Object.values(p).join(' '))
      .join(',')}`;
  }

  drawSVG(dest, src) {
    let args = [
      dest,
      {
        x: (src.x + dest.x) / 2,
        y: src.y,
      },
    ];

    args.push({
      x: args[1].x,
      y: dest.y,
    });

    return `M ${src.x} ${src.y} ${this.curve(...args)}`;
  }

  // =actions
  @action
  calculate() {
    // Calculate viewBox dimensions
    this.downView = this.getDimensions('#downstream-lines');
    this.upView = this.getDimensions('#upstream-lines');

    // Get Element Positions
    const downCards = this.getDownCards();
    const grafanaCard = this.getGrafana();
    const upCards = this.getUpCards();

    // Set center positioning points
    this.centerDimensions = this.getSVGDimensions(grafanaCard.id.toString());

    let downCalcs = [];
    downCards.forEach(item => {
      const dimensions = this.getSVGDimensions(item.id.toString());

      const dest = {
        x: this.centerDimensions.x,
        y: this.centerDimensions.y + this.centerDimensions.height / 4,
      };
      const src = {
        x: dimensions.x + dimensions.width,
        y: dimensions.y + dimensions.height / 2,
      };

      downCalcs.push({
        ...dimensions,
        line: this.drawSVG(dest, src),
        id: this.dom.guid(item),
      });
    });

    // Set Downstream Cards Positioning points
    this.downCardDimensions = downCalcs;

    let upCalcs = [];
    upCards.forEach(item => {
      const dimensions = this.getSVGDimensions(item.id.toString());
      const dest = {
        x: dimensions.x - dimensions.width - 30,
        y: dimensions.y + dimensions.height / 2,
      };
      const src = {
        x: this.centerDimensions.x,
        y: this.centerDimensions.y + this.centerDimensions.height / 4,
      };

      upCalcs.push({
        ...dimensions,
        line: this.drawSVG(dest, src),
        id: this.dom.guid(item),
      });
    });

    // Set Upstream Cards Positioning points
    this.upCardDimensions = upCalcs;
  }
}
