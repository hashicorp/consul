import validations from 'consul-ui/validations/role';
import builderFactory from 'consul-ui/utils/form/builder';
const builder = builderFactory();
export default function(container, name = 'role', v = validations, form = builder) {
  return form(name, {})
    .setValidators(v)
    .add(container.form('policy'));
}
