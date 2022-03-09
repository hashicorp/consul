export default (css) => {
  return css`
    :host {
      display: block;
      width: 100%;
      height: 100%;
    }
    dl {
      position: relative;
      height: 100%;
    }
    :host([type='linear']) {
      height: 3px;
    }
    :host([type='radial']),
    :host([type='circular']) {
      height: 300px;
    }
    :host([type='linear']) dl {
      background-color: currentColor;
      color: rgb(var(--tone-gray-100));
      border-radius: var(--decor-radius-999);
      transition-property: transform;
      transition-timing-function: ease-out;
      transition-duration: .1s;
    }
    :host([type='linear']) dl:hover {
      transform: scaleY(3);
      box-shadow: var(--decor-elevation-200);
    }
  `;
}
