import setHelpers from 'mnemonist/set';
import { andOr } from 'consul-ui/utils/filter';

export default andOr({
  kinds: {
    management: (item, value) => item.Type === value,
    client: (item, value) => item.Type === value,
  },
});
