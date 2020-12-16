import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

const templateRe = /\${([A-Za-z.0-9_-]+)}/g;
let render;
export default class UriHelper extends Helper {
  @service('encoder') encoder;
  constructor() {
    super(...arguments);
    if (typeof render !== 'function') {
      render = this.encoder.createRegExpEncoder(templateRe, encodeURIComponent);
    }
  }

  compute([template, vars]) {
    return render(template, vars);
  }
}
