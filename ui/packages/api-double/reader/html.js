module.exports = function(url) {
  const template = document.querySelector(`script[data-url="${url}"]`);
  if (template) {
    return Promise.resolve(template.textContent);
  }
  return Promise.reject();
};
