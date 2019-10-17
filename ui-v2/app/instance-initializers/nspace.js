import config from 'consul-ui/config/environment';
export function initialize(container) {
  if (config.CONSUL_NSPACES_ENABLED) {
    ['dc', 'dc.intentions.edit', 'dc.intentions.create'].forEach(function(item) {
      container.inject(`route:${item}`, 'nspaceRepo', 'service:repository/nspace/enabled');
      container.inject(`route:nspace.${item}`, 'nspaceRepo', 'service:repository/nspace/enabled');
    });
    container.inject('route:application', 'nspaceRepo', 'service:repository/nspace/enabled');
    container
      .lookup('service:dom')
      .root()
      .classList.add('has-nspaces');
  }
}

export default {
  initialize,
};
