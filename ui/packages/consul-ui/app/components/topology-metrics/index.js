import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action, get } from '@ember/object';

export default class TopologyMetrics extends Component {
  // =attributes
  @tracked centerDimensions;
  @tracked downView;
  @tracked downLines = [];
  @tracked upView;
  @tracked upLines = [];
  @tracked noMetricsReason;

  // =methods
  drawDownLines(items) {
    const order = ['allow', 'deny'];
    const dest = {
      x: this.centerDimensions.x - 7,
      y: this.centerDimensions.y + this.centerDimensions.height / 2,
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
      x: this.centerDimensions.x + 5.5,
      y: this.centerDimensions.y + this.centerDimensions.height / 2,
    };

    return items
      .map(item => {
        const dimensions = item.getBoundingClientRect();
        const dest = {
          x: dimensions.x - dimensions.width - 25,
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

  get upstreams() {
    const upstreams = get(this.args.topology, 'Upstreams') || [];
    const items = [...upstreams];
    const defaultAllow = get(this.args.topology, 'DefaultAllow');
    const wildcardIntention = get(this.args.topology, 'WildcardIntention');
    if (defaultAllow || wildcardIntention) {
      items.push({
        Name: '* (All Services)',
        Datacenter: '',
        Namespace: '',
        Intention: {
          Allowed: true,
        },
      });
    }
    return items;
  }

  get mainNotIngressService() {
    const kind = get(this.args.service.Service, 'Kind') || '';

    return kind !== 'ingress-gateway';
  }

  // =actions
  @action
  setHeight(el, item) {
    if (el) {
      const container = el.getBoundingClientRect();
      document.getElementById(`${item[0]}`).setAttribute('style', `height:${container.height}px`);
    }

    this.calculate();
  }

  @action
  calculate() {
    if (this.args.isRemoteDC) {
      this.noMetricsReason = 'remote-dc';
    } else if (this.args.service.Service.Kind === 'ingress-gateway') {
      this.noMetricsReason = 'ingress-gateway';
    } else {
      this.noMetricsReason = null;
    }

    // Calculate viewBox dimensions
    this.downView = document.getElementById('downstream-lines').getBoundingClientRect();
    const upstreamLines = document.getElementById('upstream-lines').getBoundingClientRect();
    const upstreamColumn = document.getElementById('upstream-column');

    if (upstreamColumn) {
      this.upView = {
        x: upstreamLines.x,
        y: upstreamLines.y,
        width: upstreamLines.width,
        height: upstreamColumn.getBoundingClientRect().height + 10,
      };
    }

    // Get Card elements positions
    const downCards = [
      ...document.querySelectorAll('#downstream-container .topology-metrics-card'),
    ];
    const grafanaCard = document.querySelector('.metrics-header');
    const upCards = [...document.querySelectorAll('#upstream-column .topology-metrics-card')];

    // Set center positioning points
    this.centerDimensions = grafanaCard.getBoundingClientRect();

    // Set Downstream Cards Positioning points
    this.downLines = this.drawDownLines(downCards);

    // Set Upstream Cards Positioning points
    this.upLines = this.drawUpLines(upCards);
  }
}
