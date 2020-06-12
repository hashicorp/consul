import service from 'consul-ui/sort/comparators/service';
import check from 'consul-ui/sort/comparators/check';

export function initialize(container) {
  // Service-less injection using private properties at a per-project level
  const Sort = container.resolveRegistration('service:sort');
  const comparators = {
    service: service(),
    check: check(),
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
