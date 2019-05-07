const scripts = document.getElementsByTagName('script');
const current = scripts[scripts.length - 1];

export function initialize(application) {
  const Client = application.resolveRegistration('service:client/http');
  Client.reopen({
    isCurrent: function(src) {
      return current.src === src;
    },
  });
}

export default {
  initialize,
};
