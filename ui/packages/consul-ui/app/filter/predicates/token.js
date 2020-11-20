import setHelpers from 'mnemonist/set';
import { andOr } from 'consul-ui/utils/filter';

export default andOr({
  kinds: {
    'global-management': (item, value) => item.isGlobalManagement,
    global: (item, value) => !item.Local,
    local: (item, value) => item.Local,
  },
});
