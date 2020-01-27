export function initialize(application) {
  const IvyCodeMirrorComponent = application.resolveRegistration('component:ivy-codemirror');
  // Make sure ivy-codemirror respects/maintains a `name=""` attribute
  IvyCodeMirrorComponent.reopen({
    attributeBindings: ['name'],
  });
}

export default {
  initialize,
};
