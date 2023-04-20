import Component from '@glimmer/component';

export default class FormGroup extends Component {
  get name() {
    return this.args.name;
  }
}
