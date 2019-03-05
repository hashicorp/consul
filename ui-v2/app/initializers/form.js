import token from 'consul-ui/forms/token';
import policy from 'consul-ui/forms/policy';
export function initialize(application) {
  // Service-less injection using private properties at a per-project level
  const FormBuilder = application.resolveRegistration('service:form');
  const forms = {
    token: token(),
    policy: policy(),
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
