import { isPresent as present, fillable, clickable, property } from 'ember-cli-page-object';

export const input = function() {
  return {
    present: present(),
    fillIn: fillable(),
  };
};
export const button = function() {
  return {
    disabled: property('disabled'),
    present: present(),
    click: clickable(),
  };
};
export const click = function() {
  return {
    present: present(),
    click: clickable(),
  };
};
export const options = function(options, selector = `input`) {
  return {
    option: options.reduce((prev, item, i) => {
      prev[item] = {
        present: present(),
        click: clickable(selector, { at: i }),
      };
      return prev;
    }, {}),
  };
};
