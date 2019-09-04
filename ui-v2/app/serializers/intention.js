import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY, ATTRS } from 'consul-ui/models/intention';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  attrs: ATTRS,
});
