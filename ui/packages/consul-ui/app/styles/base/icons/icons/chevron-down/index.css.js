export default css => css `
@keyframes icon-chevron-down {
  50% {

    background-color: var(--icon-color, var(--color-chevron-down-500, currentColor));
    -webkit-mask-image: var(--icon-chevron-down-24);
    mask-image: var(--icon-chevron-down-24);
  }

  100% {

    background-color: var(--icon-color, var(--color-chevron-down-500, currentColor));
    -webkit-mask-image: var(--icon-chevron-down-16);
    mask-image: var(--icon-chevron-down-16);
  }
}`;
