import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class TopologyMetrics extends Component {
  // =attributes
  tagName = '';

  @tracked centerDimensions;
  @tracked downView;
  @tracked downLines = [];
  @tracked toMetricsArrow;
  @tracked fromMetricsCircle;
  @tracked upView;
  @tracked upLines = [];

  // =methods

  curve() {
    const args = [...arguments];
    return `${arguments.length > 2 ? `C` : `Q`} ${args
      .concat(args.shift())
      .map(p => Object.values(p).join(' '))
      .join(',')}`;
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

  drawDownLines(items) {
    let calculations = [];
    items.forEach(item => {
      const dimensions = this.getSVGDimensions(item);
      const dest = {
        x: this.centerDimensions.x,
        y: this.centerDimensions.y + this.centerDimensions.height / 4,
      };
      const src = {
        x: dimensions.x + dimensions.width,
        y: dimensions.y + dimensions.height / 2,
      };

      calculations.push({
        ...dimensions,
        line: this.drawLine(dest, src),
        id: item.id
          .split('-')
          .slice(1)
          .join('-'),
      });
    });

    return calculations;
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

  drawMetricsArrow(src) {
    // The top/bottom points have the same X position
    const x = src.x - 3;
    const topY = src.y + src.height * 0.25 - 5;
    const bottomY = src.y + src.height * 0.25 + 5;

    const middleX = src.x + 7;
    const middleY = src.y + src.height / 4;

    return `${x} ${topY} ${middleX} ${middleY} ${x} ${bottomY}`;
  }

  drawMetricsCircle(src) {
    return {
      x: src.x + 20,
      y: src.y + src.height / 4,
    };
  }

  drawUpLines(items) {
    let calculations = [];
    items.forEach(item => {
      const dimensions = this.getSVGDimensions(item);
      const dest = {
        x: dimensions.x - dimensions.width - 30,
        y: dimensions.y + dimensions.height / 2,
      };
      const src = {
        x: this.centerDimensions.x + 20,
        y: this.centerDimensions.y + this.centerDimensions.height / 4,
      };

      calculations.push({
        ...dimensions,
        line: this.drawLine(dest, src),
        // Draws the arrow that goes from Metrics -> Upstream
        arrow: this.drawArrowToUpstream(dimensions),
        id: item.id
          .split('-')
          .slice(1)
          .join('-'),
      });
    });

    return calculations;
  }

  getSVGDimensions(item) {
    const $el = item;
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

  // =actions
  @action
  calculate(e) {
    // Calculate viewBox dimensions
    this.downView = document.querySelector('#downstream-lines').getBoundingClientRect();
    this.upView = document.querySelector('#upstream-lines').getBoundingClientRect();

    // Get Card elements positions
    const downCards = document.querySelectorAll('[id^="downstreamCard"]');
    const grafanaCard = document.querySelector('#metrics-container');
    const upCards = document.querySelectorAll('[id^="upstreamCard"]');

    // Set center positioning points
    this.centerDimensions = this.getSVGDimensions(grafanaCard);

    // Set Downstream Cards Positioning points
    this.downLines = this.drawDownLines(downCards);

    // Draw arrow that goes from Downstreams -> Metrics
    this.toMetricsArrow = this.drawMetricsArrow(this.centerDimensions);

    // Set Upstream Cards Positioning points
    this.upLines = this.drawUpLines(upCards);

    // Draw the circle Metrics -> Upstreams
    this.fromMetricsCircle = this.drawMetricsCircle(this.centerDimensions);
  }
}
