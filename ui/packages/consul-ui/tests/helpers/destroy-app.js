import { run } from '@ember/runloop';

export default function destroyApp(application) {
  run(application, 'destroy');
  if (window.server) {
    window.server.shutdown();
  }
}
