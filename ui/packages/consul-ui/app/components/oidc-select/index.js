import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

import chart from './chart.xstate';

export default class OidcSelect extends Component {
  @tracked partition = '';
  constructor() {
    super(...arguments);
    this.chart = chart;
  }
}
