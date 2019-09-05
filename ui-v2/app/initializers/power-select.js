export function initialize(application) {
  const PowerSelectComponent = application.resolveRegistration('component:power-select');
  PowerSelectComponent.reopen({
    updateState: function(changes) {
      if (!this.isDestroyed) {
        return this._super(changes);
      }
    },
  });
}

export default {
  initialize,
};
