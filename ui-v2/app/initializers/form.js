import kv from 'consul-ui/forms/kv';
import acl from 'consul-ui/forms/acl';
import token from 'consul-ui/forms/token';
import policy from 'consul-ui/forms/policy';
import intention from 'consul-ui/forms/intention';
export function initialize(application) {
  // Service-less injection using private properties at a per-project level
  const FormBuilder = application.resolveRegistration('service:form');
  const forms = {
    kv: kv(),
    acl: acl(),
    token: token(),
    policy: policy(),
    intention: intention(),
  };
  FormBuilder.reopen({
    form: function(name) {
      return forms[name];
    },
  });
}

export default {
  initialize,
};
