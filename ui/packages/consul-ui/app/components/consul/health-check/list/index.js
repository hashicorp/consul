import Component from '@ember/component';

export default Component.extend({
  actions: {
    async reRunCheck(checkId) {
      await fetch("/v1/agent/check/run?check-id=" + checkId);
    }
  }
})
