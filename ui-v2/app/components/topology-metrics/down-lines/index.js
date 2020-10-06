import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class TopoloyMetricsLines extends Component {
  @tracked iconPositions;

  @action
  getIconPositions() {
    const view = this.args.view;
    const lines = [...document.querySelectorAll('#downstream-lines path')];

    this.iconPositions = lines.map(item => {
      const pathLen = parseFloat(item.getTotalLength());
      const thirdLen = item.getPointAtLength(Math.ceil(pathLen / 3));

      return {
        id: item.id,
        x: thirdLen.x - view.x,
        y: thirdLen.y - view.y,
      };
    });
  }
}
