import NavigatorClipboard from 'consul-ui/services/clipboard/native';
import ExecClipboard from 'consul-ui/services/clipboard/polyfill';

export function initialize(application) {
  // which clipboard impl. depends on whether we have native support
  if (!application.hasRegistration('service:clipboard')) {
    application.register(
      `service:clipboard`,
      window.navigator.clipboard ? NavigatorClipboard : ExecClipboard
    );
  }
}

export default {
  initialize,
};
