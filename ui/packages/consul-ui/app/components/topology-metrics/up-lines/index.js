import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class TopologyMetricsUpLines extends Component {
  @tracked iconPositions;
  @service('dom') dom;

  get guid() {
    return this.dom.guid(this);
  }

  @action
  getIconPositions() {
    const center = this.args.center;
    const view = this.args.view;
    const lines = [...document.querySelectorAll('#upstream-lines path')];

    this.iconPositions = lines.map((item) => {
      const pathLen = parseFloat(item.getTotalLength());
      const partLen = item.getPointAtLength(Math.ceil(pathLen * 0.666));
      return {
        id: item.id,
        x: Math.round(partLen.x - center.x),
        y: Math.round(partLen.y - view.y),
      };
    });
  }
}
