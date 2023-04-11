const parseFloatWithDefault = (val, d = 0) => {
  const num = parseFloat(val);
  return isNaN(num) ? d : num;
};

export default (Component) => {
  return class extends Component {
    attributeChangedCallback(name, prev, value) {
      const target = this;
      switch (name) {
        case 'percentage': {
          let prevSibling = target;
          while (prevSibling) {
            const nextSibling = prevSibling.nextElementSibling;
            const aggregatedPercentage = nextSibling
              ? parseFloatWithDefault(nextSibling.style.getPropertyValue('--aggregated-percentage'))
              : 0;
            const perc =
              parseFloatWithDefault(prevSibling.getAttribute('percentage')) + aggregatedPercentage;
            prevSibling.style.setProperty('--aggregated-percentage', perc);
            prevSibling.setAttribute('aggregated-percentage', perc);
            prevSibling = prevSibling.previousElementSibling;
          }
          break;
        }
      }
    }
  };
};
