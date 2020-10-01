import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class TopologyMetrics extends Component {
  // =attributes
  @tracked centerDimensions;
  @tracked downView;
  @tracked downLines = [];
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

  drawUpLines(items) {
    let calculations = [];
    items.forEach(item => {
      const dimensions = this.getSVGDimensions(item);
      const dest = {
        x: dimensions.x - dimensions.width - 26,
        y: dimensions.y + dimensions.height / 2,
      };
      const src = {
        x: this.centerDimensions.x + 20,
        y: this.centerDimensions.y + this.centerDimensions.height / 4,
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
  calculate() {
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

    // Set Upstream Cards Positioning points
    this.upLines = this.drawUpLines(upCards);
  }
}
