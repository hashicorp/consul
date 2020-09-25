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
  @tracked toMetricsArrow;

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

  drawLine(dest, src) {
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

  drawArrowToMetrics(dest) {
    // The top/bottom points have the same X position
    const x = dest.x - 3;
    const topY = dest.y + dest.height * 0.25 - 5;
    const bottomY = dest.y + dest.height * 0.25 + 5;

    const middleX = dest.x + 7;
    const middleY = dest.y + dest.height / 4;

    return `${x} ${topY} ${middleX} ${middleY} ${x} ${bottomY}`;
  }

  drawArrowToUpstream(dest) {
    // The top/bottom points have the same X position
    const x = dest.x - dest.width - 31;
    const topY = dest.y + dest.height / 2 - 5;
    const bottomY = dest.y + dest.height / 2 + 5;

    const middleX = dest.x - dest.width - 21;
    const middleY = dest.y + dest.height / 2;

    return `${x} ${topY} ${middleX} ${middleY} ${x} ${bottomY}`;
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

    // Draws the arrow that goes from Downstreams -> Metrics
    this.toMetricsArrow = this.drawArrowToMetrics(this.centerDimensions);

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
        line: this.drawLine(dest, src),
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
        line: this.drawLine(dest, src),
        // Draws the arrow that goes from Metrics -> Upstream
        arrow: this.drawArrowToUpstream(dimensions),
        id: this.dom.guid(item),
      });
    });

    // Set Upstream Cards Positioning points
    this.upCardDimensions = upCalcs;
  }
}
