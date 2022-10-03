export default (css) => {
  return css`
    /*@import '~/styles/base/decoration/visually-hidden.css';*/

    :host(.critical) {
      color: rgb(var(--tone-red-500));
    }
    :host(.warning) {
      color: rgb(var(--tone-orange-500));
    }
    :host(.passing) {
      color: rgb(var(--tone-green-500));
    }

    :host {
      position: absolute;
      top: 0;
      height: 100%;

      transition-timing-function: ease-out;
      transition-duration: 0.5s;
    }
    dt,
    dd meter {
      animation-name: visually-hidden;
      animation-fill-mode: forwards;
      animation-play-state: paused;
    }

    :host(.type-linear) {
      transition-property: width;
      width: calc(var(--aggregated-percentage) * 1%);
      height: 100%;
      background-color: currentColor;
      border-radius: var(--decor-radius-999);
    }

    :host svg {
      height: 100%;
    }
    :host(.type-radial),
    :host(.type-circular) {
      transition-property: none;
    }
    :host(.type-radial) dd,
    :host(.type-circular) dd {
      width: 100%;
      height: 100%;
    }
    :host(.type-radial) circle,
    :host(.type-circular) circle {
      transition-timing-function: ease-out;
      transition-duration: 0.5s;
      pointer-events: stroke;
      transition-property: stroke-dashoffset, stroke-width;
      transform: rotate(-90deg);
      transform-origin: 50%;
      fill: transparent;
      stroke: currentColor;
      stroke-dasharray: 100, 100;
      stroke-dashoffset: calc(calc(100 - var(--aggregated-percentage)) * 1px);
    }
    :host([aggregated-percentage='100']) circle {
      stroke-dasharray: 0 !important;
    }
    :host([aggregated-percentage='0']) circle {
      stroke-dasharray: 0, 100 !important;
    }
    :host(.type-radial) circle,
    :host(.type-circular]) svg {
      pointer-events: none;
    }
    :host(.type-radial) circle {
      stroke-width: 32;
    }
    :host(.type-circular) circle {
      stroke-width: 14;
    }
  `;
};
