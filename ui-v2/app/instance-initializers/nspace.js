import config from 'consul-ui/config/environment';
export function initialize(container) {
  if (config.CONSUL_NSPACES_ENABLED) {
    ['dc', 'settings', 'dc.intentions.edit', 'dc.intentions.create'].forEach(function(item) {
      container.inject(`route:${item}`, 'nspacesRepo', 'service:repository/nspace/enabled');
      container.inject(`route:nspace.${item}`, 'nspacesRepo', 'service:repository/nspace/enabled');
    });
    container.inject('route:application', 'nspacesRepo', 'service:repository/nspace/enabled');
    container
      .lookup('service:dom')
      .root()
      .classList.add('has-nspaces');
  }
  // FIXME: This needs to live in its own initializer, either:
  // 1. Make it be about adding classes to the root dom node
  // 2. Make it be about config and things to do on initialization re: config
  // If we go with 1 then we need to move both this and the above nspaces class
  if (config.CONSUL_ACLS_ENABLED) {
    container
      .lookup('service:dom')
      .root()
      .classList.add('has-acls');
  }
}

export default {
  initialize,
};
