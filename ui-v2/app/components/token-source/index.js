import Component from '@ember/component';

import chart from './chart.xstate';
export default Component.extend({
  onchange: function() {},
  init: function() {
    this._super(...arguments);
    this.chart = chart;
  },
  actions: {
    isSecret: function() {
      return this.type === 'secret';
    },
    change: function(e) {
      e.data.toJSON = function() {
        return {
          AccessorID: this.AccessorID,
          // TODO: In the past we've always ignored the SecretID returned
          // from the server and used what the user typed in instead, now
          // as we don't know the SecretID when we use SSO we use the SecretID
          // in the response
          SecretID: this.SecretID,
          Namespace: this.Namespace,
          ...{
            AuthMethod: typeof this.AuthMethod !== 'undefined' ? this.AuthMethod : undefined,
            // TODO: We should be able to only set namespaces if they are enabled
            // but we might be testing for nspaces everywhere
            // Namespace: typeof this.Namespace !== 'undefined' ? this.Namespace : undefined
          },
        };
      };
      // FIXME: We should probably put the component into idle state
      this.onchange(e);
    },
  },
});
