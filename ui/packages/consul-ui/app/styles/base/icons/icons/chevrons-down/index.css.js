export default css => css `
@keyframes icon-chevrons-down {
  50% {

    background-color: var(--icon-color, var(--color-chevrons-down-500, currentColor));
    -webkit-mask-image: var(--icon-chevrons-down-24);
    mask-image: var(--icon-chevrons-down-24);
  }

  100% {

    background-color: var(--icon-color, var(--color-chevrons-down-500, currentColor));
    -webkit-mask-image: var(--icon-chevrons-down-16);
    mask-image: var(--icon-chevrons-down-16);
  }
}
`;
