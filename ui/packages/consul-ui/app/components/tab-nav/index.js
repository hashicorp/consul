import Component from '@glimmer/component';

function noop() {}
export default class TabNav extends Component {
  get onClick() {
    return this.args.onclick || noop;
  }

  get onTabClicked() {
    return this.args.onTabClicked || noop;
  }
}
