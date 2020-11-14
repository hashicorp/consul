import { helper } from '@ember/component/helper';
import { env } from 'consul-ui/env';
export default helper(function([name, def = ''], hash) {
  const val = env(name);
  return val != null ? val : def;
});
