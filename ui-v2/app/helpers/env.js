import { helper } from '@ember/component/helper';
import $ from 'consul-ui/config/environment';
export function env([name, def = ''], hash) {
  return $[name] != null ? $[name] : def;
}

export default helper(env);
