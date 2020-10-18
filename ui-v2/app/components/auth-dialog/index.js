import Component from '@ember/component';
import Slotted from 'block-slots';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import chart from './chart.xstate';

export default Component.extend(Slotted, {
  tagName: '',
  repo: service('repository/oidc-provider'),
  init: function() {
    this._super(...arguments);
    this.chart = chart;
  },
  actions: {
    hasToken: function() {
      return typeof this.token.AccessorID !== 'undefined';
    },
    login: function() {
      let prev = get(this, 'previousToken.AccessorID');
      let current = get(this, 'token.AccessorID');
      if (prev === null) {
        prev = get(this, 'previousToken.SecretID');
      }
      if (current === null) {
        current = get(this, 'token.SecretID');
      }
      let type = 'authorize';
      if (typeof prev !== 'undefined' && prev !== current) {
        type = 'use';
      }
      this.onchange({ data: get(this, 'token'), type: type });
    },
    logout: function() {
      if (typeof get(this, 'previousToken.AuthMethod') !== 'undefined') {
        // we are ok to fire and forget here
        this.repo.logout(get(this, 'previousToken.SecretID'));
      }
      this.previousToken = null;
      this.onchange({ data: null, type: 'logout' });
    },
  },
});
