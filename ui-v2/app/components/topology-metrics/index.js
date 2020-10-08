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
  drawDownLines(items) {
    const order = ['allow', 'deny'];
    const dest = {
      x: this.centerDimensions.x,
      y: this.centerDimensions.y + this.centerDimensions.height / 4,
    };

    return items
      .map(item => {
        const dimensions = item.getBoundingClientRect();
        const src = {
          x: dimensions.x + dimensions.width,
          y: dimensions.y + dimensions.height / 2,
        };

        return {
          id: item.id,
          permission: item.getAttribute('data-permission'),
          dest: dest,
          src: src,
        };
      })
      .sort((a, b) => {
        return order.indexOf(a.permission) - order.indexOf(b.permission);
      });
  }

  drawUpLines(items) {
    const order = ['allow', 'deny'];
    const src = {
      x: this.centerDimensions.x + 20,
      y: this.centerDimensions.y + this.centerDimensions.height / 4,
    };

    return items
      .map(item => {
        const dimensions = item.getBoundingClientRect();
        const dest = {
          x: dimensions.x - dimensions.width - 26,
          y: dimensions.y + dimensions.height / 2,
        };

        return {
          id: item.id,
          permission: item.getAttribute('data-permission'),
          dest: dest,
          src: src,
        };
      })
      .sort((a, b) => {
        return order.indexOf(a.permission) - order.indexOf(b.permission);
      });
  }

  // =actions
  @action
  calculate() {
    // Calculate viewBox dimensions
    this.downView = document.querySelector('#downstream-lines').getBoundingClientRect();
    this.upView = document.querySelector('#upstream-lines').getBoundingClientRect();

    // Get Card elements positions
    const downCards = [...document.querySelectorAll('#downstream-container .card')];
    const grafanaCard = document.querySelector('#metrics-container');
    const upCards = [...document.querySelectorAll('#upstream-column .card')];

    // Set center positioning points
    this.centerDimensions = grafanaCard.getBoundingClientRect();

    // Set Downstream Cards Positioning points
    this.downLines = this.drawDownLines(downCards);

    // Set Upstream Cards Positioning points
    this.upLines = this.drawUpLines(upCards);
  }
}
