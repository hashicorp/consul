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
      const secret = this.value;
      e.data.toJSON = function() {
        return {
          AccessorID: this.AccessorID,
          // TODO: In the past we've always ignored the SecretID returned
          // from the server and used what the user typed in instead
          // is this still the preferred thing to do?
          SecretID: secret, //this.SecretID,
          Namespace: this.Namespace,
          ...{
            AuthMethod: typeof this.AuthMethod !== 'undefined' ? this.AuthMethod : undefined,
            // Namespace: typeof this.Namespace !== 'undefined' ? this.Namespace : undefined
          },
        };
      };
      // FIXME: We should probably put the component into idle state
      this.onchange(e);
    },
  },
});
