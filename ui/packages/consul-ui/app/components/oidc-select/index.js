import Component from '@glimmer/component';
import chart from './chart.xstate';

export default class OidcSelect extends Component {
  constructor() {
    super(...arguments);
    this.chart = chart;
  }
}
