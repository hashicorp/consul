import { get } from '@ember/object';
export function initialize(application) {
  const PowerSelectComponent = application.resolveRegistration('component:power-select');
  PowerSelectComponent.reopen({
    updateState: function(changes) {
      if (!get(this, 'isDestroyed')) {
        return this._super(changes);
      }
    },
  });
}

export default {
  initialize,
};
