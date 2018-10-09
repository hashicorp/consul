import builderFactory from 'consul-ui/utils/form/builder';
import validations from 'consul-ui/validations/token';
import policy from 'consul-ui/forms/policy';
const builder = builderFactory();
export default function(name = '', v = validations, form = builder) {
  return form(name, {})
    .setValidators(v)
    .add(policy());
}
