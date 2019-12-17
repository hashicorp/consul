import { helper } from '@ember/component/helper';
import { config } from 'consul-ui/env';
// TODO: env actually uses config values not env values
// see `app/env` for the renaming TODO's also
export function env([name, def = ''], hash) {
  return config(name) != null ? config(name) : def;
}

export default helper(env);
