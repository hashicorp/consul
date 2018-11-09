import validations from 'consul-ui/validations/token';
import policy from 'consul-ui/forms/policy';
import builderFactory from 'consul-ui/utils/form/builder';
const builder = builderFactory();
export default function(name = '', v = validations, form = builder) {
  return form(name, {})
    .setValidators(v)
    .add(policy());
}
