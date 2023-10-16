import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class TopoloyMetricsDownLines extends Component {
  @tracked iconPositions;
  @service('dom') dom;

  get guid() {
    return this.dom.guid(this);
  }

  @action
  getIconPositions() {
    const view = this.args.view;
    const lines = [...document.querySelectorAll('#downstream-lines path')];

    this.iconPositions = lines.map((item) => {
      const pathLen = parseFloat(item.getTotalLength());
      const thirdLen = item.getPointAtLength(Math.ceil(pathLen / 3));

      return {
        id: item.id,
        x: Math.round(thirdLen.x - view.x),
        y: Math.round(thirdLen.y - view.y),
      };
    });
  }
}
