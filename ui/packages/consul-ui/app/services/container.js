import Service from '@ember/service';

export default class ContainerService extends Service {
  constructor(owner) {
    super(...arguments);
    this._owner = owner;
    this._wm = new WeakMap();
  }

  set(key, value) {
    this._wm.set(value, key);
  }

  // vaguely private, used publicly for debugging purposes
  keyForClass(cls) {
    return this._wm.get(cls);
  }

  get(key) {
    if (typeof key !== 'string') {
      key = this.keyForClass(key);
    }
    return this.lookup(key);
  }

  lookup(key) {
    return this._owner.lookup(key);
  }

  resolveRegistration(key) {
    // ember resolveRegistration returns an ember flavoured class extending
    // from the actual class, access the actual class from the
    // prototype/parent which is what decorators pass through as target
    return this._owner.resolveRegistration(key).prototype;
  }
}
