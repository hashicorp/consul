import Helper from '@ember/component/helper';
import { css } from '@lit/reactive-element';

export default class ConsoleLogHelper extends Helper {
  compute([str], hash) {
    return css([str]);
  }
}
