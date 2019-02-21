export default function(scenario, fillIn, triggerKeyEvent, currentPage) {
  const fillInElement = function(page, name, value) {
    const cm = document.querySelector(`textarea[name="${name}"] + .CodeMirror`);
    if (cm) {
      cm.CodeMirror.setValue(value);
      return page;
    } else {
      return page.fillIn(name, value);
    }
  };
  scenario
    .when('I submit', function(selector) {
      return currentPage().submit();
    })
    .then('I fill in "$name" with "$value"', function(name, value) {
      return currentPage().fillIn(name, value);
    })
    .then(['I fill in with yaml\n$yaml', 'I fill in with json\n$json'], function(data) {
      return Object.keys(data).reduce(function(prev, item, i, arr) {
        return fillInElement(prev, item, data[item]);
      }, currentPage());
    })
    .then(
      ['I fill in the $form form with yaml\n$yaml', 'I fill in the $form with json\n$json'],
      function(form, data) {
        return Object.keys(data).reduce(function(prev, item, i, arr) {
          const name = `${form}[${item}]`;
          return fillInElement(prev, name, data[item]);
        }, currentPage());
      }
    )
    .then(['I type "$text" into "$selector"'], function(text, selector) {
      return fillIn(selector, text);
    })
    .then(['I type with yaml\n$yaml'], function(data) {
      const keys = Object.keys(data);
      return keys
        .reduce(function(prev, item, i, arr) {
          return prev.fillIn(item, data[item]);
        }, currentPage())
        .then(function() {
          return Promise.all(
            keys.map(function(item) {
              return triggerKeyEvent(`[name="${item}"]`, 'keyup', 83); // TODO: This is 's', be more generic
            })
          );
        });
    });
}
