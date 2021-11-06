import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

// regexp that matches {{item.Name}} or ${item.Name}
// what this regex does
// (?:\$|\{)            - Match either $ or {
// \{                   - Match {
// ([a-z.0-9_-]+)       - Capturing group
// (?:(?<=\$\{[^{]+)    - Use a positive lookbehind to assert that ${ was matched previously
//   |\}            )   - or match a }
// \}                   - Match }
const templateRe = /(?:\$|\{)\{([a-z.0-9_-]+)(?:(?<=\$\{[^{]+)|\})\}/gi;
let render;
export default class RenderTemplateHelper extends Helper {
  @service('encoder') encoder;
  constructor() {
    super(...arguments);
    if (typeof render !== 'function') {
      render = this.encoder.createRegExpEncoder(templateRe, encodeURIComponent, false);
    }
  }

  compute([template, vars]) {
    return render(template, vars);
  }
}
