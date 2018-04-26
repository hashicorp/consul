import Serializer from './application';
import { typeOf } from '@ember/utils';
import { get } from '@ember/object';
import { PRIMARY_KEY } from 'consul-ui/models/acl';
export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
});
