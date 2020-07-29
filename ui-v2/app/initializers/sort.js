import service from 'consul-ui/sort/comparators/service';
import check from 'consul-ui/sort/comparators/check';
import intention from 'consul-ui/sort/comparators/intention';
import token from 'consul-ui/sort/comparators/token';
import role from 'consul-ui/sort/comparators/role';

export function initialize(container) {
  // Service-less injection using private properties at a per-project level
  const Sort = container.resolveRegistration('service:sort');
  const comparators = {
    service: service(),
    check: check(),
    intention: intention(),
    token: token(),
    role: role(),
  };
  Sort.reopen({
    comparator: function(type) {
      return comparators[type];
    },
  });
}

export default {
  initialize,
};
