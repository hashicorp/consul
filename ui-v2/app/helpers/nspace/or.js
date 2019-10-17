import { helper } from '@ember/component/helper';
import config from 'consul-ui/config/environment';

export default helper(function nspaceOr([nspace], hash) {
  return nspace || config.CONSUL_NSPACES_UNDEFINED_NAME;
});
