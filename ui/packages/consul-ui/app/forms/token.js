import validations from 'consul-ui/validations/token';
import builderFactory from 'consul-ui/utils/form/builder';
const builder = builderFactory();
export default function (container, name = '', v = validations, form = builder) {
  return form(name, {}).setValidators(v).add(container.form('policy')).add(container.form('role'));
}
