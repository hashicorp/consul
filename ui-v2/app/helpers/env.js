import { helper } from '@ember/component/helper';
import $ from 'consul-ui/config/environment';
export function env([name], hash) {
  return $[name];
}

export default helper(env);
