export default function(scenario, find, fillIn, triggerKeyEvent, currentPage) {
  const dont = `( don't| shouldn't| can't)?`;
  const fillInElement = function(page, name, value) {
    const cm = document.querySelector(`textarea[name="${name}"] + .CodeMirror`);
    if (cm) {
      if (!cm.CodeMirror.options.readOnly) {
        cm.CodeMirror.setValue(value);
      } else {
        throw new Error(`The ${name} editor is set to readonly`);
      }
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
      [
        `I${dont} fill in the $property form with yaml\n$yaml`,
        `I${dont} fill in $property with yaml\n$yaml`,
        `I${dont} fill in the $property with yaml\n$yaml`,
        `I${dont} fill in the property form with json\n$json`,

        `I${dont} fill in the $property form on the $component component with yaml\n$yaml`,
        `I${dont} fill in the $property form on the $component component with json\n$json`,
        `I${dont} fill in the $property on the $component component with yaml\n$yaml`,
      ],
      function(negative, property, component, data, next) {
        try {
          switch (true) {
            case typeof component === 'string':
              property = `${component}.${property}`;
            // fallthrough
            case typeof data === 'undefined':
              data = component;
            // // fallthrough
            // case typeof property !== 'string':
            // data = property;
          }
          let obj;
          try {
            obj = find(property);
          } catch (e) {
            obj = currentPage();
          }
          return Object.keys(data).reduce(function(prev, item, i, arr) {
            const name = `${obj.prefix || property}[${item}]`;
            if (negative) {
              try {
                fillInElement(prev, name, data[item]);
                throw new TypeError(`${item} is editable`);
              } catch (e) {
                if (e instanceof TypeError) {
                  throw e;
                }
              }
            } else {
              return fillInElement(prev, name, data[item]);
            }
          }, obj);
        } catch (e) {
          throw e;
        }
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
