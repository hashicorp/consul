import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class TopologyMetrics extends Component {
  @service('ui-config') cfg;
  @service('env') env;

  // =attributes
  @tracked centerDimensions;
  @tracked downView;
  @tracked downLines = [];
  @tracked upView;
  @tracked upLines = [];
  @tracked hasMetricsProvider = false;
  @tracked noMetricsReason = null;

  constructor(owner, args) {
    super(owner, args);
    this.hasMetricsProvider = !!this.cfg.get().metrics_provider;

    // Disable metrics fetching if we are not in the local DC since we don't
    // currently support that for all providers.
    //
    // TODO we can make the configurable even before we have a full solution for
    // multi-DC forwarding for Prometheus so providers that are global for all
    // DCs like an external managed APM can still load in all DCs.
    if (
      this.env.var('CONSUL_DATACENTER_LOCAL') !== this.args.topology.get('Datacenter') ||
      this.args.service.Service.Kind === 'ingress-gateway'
    ) {
      this.noMetricsReason = 'Unable to fetch metrics for a remote datacenter';
    }
  }

  // =methods
  drawDownLines(items) {
    const order = ['allow', 'deny'];
    const dest = {
      x: this.centerDimensions.x,
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
      x: this.centerDimensions.x,
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

  // =actions
  @action
  calculate() {
    // Calculate viewBox dimensions
    this.downView = document.querySelector('#downstream-lines').getBoundingClientRect();
    this.upView = document.querySelector('#upstream-lines').getBoundingClientRect();

    // Get Card elements positions
    const downCards = [...document.querySelectorAll('#downstream-container .card')];
    const grafanaCard = document.querySelector('.metrics-header');
    const upCards = [...document.querySelectorAll('#upstream-column .card')];

    // Set center positioning points
    this.centerDimensions = grafanaCard.getBoundingClientRect();

    // Set Downstream Cards Positioning points
    this.downLines = this.drawDownLines(downCards);

    // Set Upstream Cards Positioning points
    this.upLines = this.drawUpLines(upCards);
  }
}
