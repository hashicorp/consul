import Serializer from './application';
import { PRIMARY_KEY } from 'consul-ui/models/kv';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
});
