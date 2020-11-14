import validations from 'consul-ui/validations/acl';
import builderFactory from 'consul-ui/utils/form/builder';
const builder = builderFactory();
export default function(container, name = '', v = validations, form = builder) {
  return form(name, {}).setValidators(v);
}
