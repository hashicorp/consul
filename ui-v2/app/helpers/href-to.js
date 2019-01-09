// This helper requires `ember-href-to` for the moment at least
// It's the same code, just with added urlencoding, see comment
// further down
import Helper from '@ember/component/helper';
import { hrefTo } from 'ember-href-to/helpers/href-to';
import urlEncode from 'consul-ui/utils/url-encode';
const encode = urlEncode(encodeURIComponent);
export default Helper.extend({
  compute([targetRouteName, ...rest], namedArgs) {
    if (namedArgs.params) {
      return hrefTo(this, ...namedArgs.params);
    } else {
      // ...rest is wrapped with encode here
      return hrefTo(this, targetRouteName, ...encode(rest));
      // return hrefTo(this, targetRouteName, ...rest);
    }
  },
});
