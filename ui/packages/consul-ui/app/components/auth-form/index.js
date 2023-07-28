import Component from '@glimmer/component';
import { action } from '@ember/object';

import chart from './chart.xstate';
import tabs from './tabs.xstate';

export default class AuthForm extends Component {
  constructor() {
    super(...arguments);
    this.chart = chart;
    this.tabsChart = tabs;
  }

  @action
  hasValue(context, event, meta) {
    return this.value !== '' && typeof this.value !== 'undefined';
  }

  @action
  focus() {
    this.input.focus();
  }
}
