import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/binding-rule';

export default class BindingRuleSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;
}
