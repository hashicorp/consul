/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (scenario, find, fillIn, triggerKeyEvent, currentPage) {
  const dont = `( don't| shouldn't| can't)?`;

  const fillInCodeEditor = function (page, name, value) {
    const valueElement = document.querySelector(`[aria-label="${name}"]`);
    const codeEditorElement = document.querySelector('.cm-editor');
    const codeBlockElement = document.querySelector('.hds-code-block');

    /**
     * if codeEditorElement is parent of valueElement, then we are dealing with a CodeMirror editor
     *
     * if codeBlockElement is parent of valueElement, then we are dealing with a HDS CodeBlock, which is readOnly
     *
     */
    if (codeEditorElement && codeEditorElement.contains(valueElement)) {
      const valueBlock = document.createElement('div');
      valueBlock.innerHTML = value;
      valueElement.appendChild(valueBlock);
    } else {
      if (codeBlockElement && codeBlockElement.contains(valueElement)) {
        throw new Error(`The ${name} editor is set to readonly`);
      }

      return page;
    }

    return page;
  };

  const fillInElement = async function (page, name, value) {
    const cm = document.querySelector(`textarea[name="${name}"] + .CodeMirror`);
    if (cm) {
      if (!cm.CodeMirror.options.readOnly) {
        cm.CodeMirror.setValue(value);
      } else {
        throw new Error(`The ${name} editor is set to readonly`);
      }
      return page;
    } else {
      const $el = document.querySelector(`[name="${name}"]`);
      await fillIn($el, value);
      return page;
    }
  };
  scenario
    .when('I submit', function (selector) {
      return currentPage().submit();
    })
    .then('I fill in "$name" with "$value"', function (name, value) {
      return currentPage().fillIn(name, value);
    })
    .then(['I fill in with yaml\n$yaml', 'I fill in with json\n$json'], async function (data) {
      const res = Object.keys(data).reduce(function (prev, item, i, arr) {
        return fillInElement(prev, item, data[item]);
      }, currentPage());
      await new Promise((resolve) => setTimeout(resolve, 0));
      return res;
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
      async function (negative, property, component, data, next) {
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
        const res = Object.keys(data).reduce(async function (prev, item, i, arr) {
          await prev;

          const name = `${obj.prefix || property}[${item}]`;
          if (negative) {
            try {
              await fillInElement(obj, name, data[item]);
              throw new TypeError(`${item} is editable`);
            } catch (e) {
              if (e instanceof TypeError) {
                throw e;
              }
            }
          } else {
            return await fillInElement(obj, name, data[item]);
          }
        }, Promise.resolve());
        return res;
      }
    )
    .then(['I fill in code editor "$name" with "$value"'], function (name, value) {
      return fillInCodeEditor(currentPage(), name, value);
    })
    .then(['I type "$text" into "$selector"'], function (text, selector) {
      return fillIn(selector, text);
    })
    .then(['I type with yaml\n$yaml'], function (data) {
      const keys = Object.keys(data);
      return keys
        .reduce(function (prev, item, i, arr) {
          return prev.fillIn(item, data[item]);
        }, currentPage())
        .then(function () {
          return Promise.all(
            keys.map(function (item) {
              return triggerKeyEvent(`[name="${item}"]`, 'keyup', 83); // TODO: This is 's', be more generic
            })
          );
        });
    });
}
