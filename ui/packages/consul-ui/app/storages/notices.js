import { TrackedArray } from 'tracked-built-ins';
import Storage from './base';

export default class Notices extends Storage {
  initState() {
    const { key, storage } = this;

    const persisted = storage.getItem(key);

    if (persisted) {
      return new TrackedArray(persisted.split(','));
    } else {
      return new TrackedArray();
    }
  }

  add(value) {
    const { key, storage, state } = this;

    state.push(value);

    storage.setItem(key, [...state]);
  }
}
