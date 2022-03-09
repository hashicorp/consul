import { helper } from '@ember/component/helper';

import { css } from '@lit/reactive-element';

import resolve from 'consul-ui/utils/path/resolve';

import distributionMeter from 'consul-ui/components/distribution-meter/index.css';
import distributionMeterMeter from 'consul-ui/components/distribution-meter/meter/index.css';
import distributionMeterMeterElement from 'consul-ui/components/distribution-meter/meter/element';
import visuallyHidden from 'consul-ui/styles/base/decoration/visually-hidden.css';

const fs = {
  ['/components/distribution-meter/index.css']: distributionMeter,
  ['/components/distribution-meter/meter/index.css']: distributionMeterMeter,
  ['/components/distribution-meter/meter/element']: distributionMeterMeterElement,
  ['/styles/base/decoration/visually-hidden.css']: visuallyHidden
};

const container = new Map();

// `css` already has a caching mechanism under the hood so rely on that, plus
// we get the advantage of laziness here, i.e. we only call css as and when we
// need to
export default helper(([path = ''], { from }) => {
  const fullPath = resolve(from, path);
  switch(true) {
    case fullPath.endsWith('.css'):
      return fs[fullPath](css)
    default: {
      if(container.has(fullPath)) {
        return container.get(fullPath);
      }
      const module = fs[fullPath](HTMLElement);
      container.set(fullPath, module);
      return module;
    }
  }
});
