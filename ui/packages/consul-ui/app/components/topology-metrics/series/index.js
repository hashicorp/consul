import Component from '@ember/component';
import dayjs from 'dayjs';
import Calendar from 'dayjs/plugin/calendar';

import { select, pointer } from 'd3-selection';
import { scaleLinear, scaleTime, scaleOrdinal } from 'd3-scale';
import { schemeTableau10 } from 'd3-scale-chromatic';
import { area, stack, stackOrderReverse } from 'd3-shape';
import { max, extent, bisector } from 'd3-array';
import { set } from '@ember/object';

dayjs.extend(Calendar);

function niceTimeWithSeconds(d) {
  return dayjs(d).calendar(null, {
    sameDay: '[Today at] h:mm:ss A',
    lastDay: '[Yesterday at] h:mm:ss A',
    lastWeek: '[Last] dddd at h:mm:ss A',
    sameElse: 'MMM DD at h:mm:ss A',
  });
}

export default Component.extend({
  data: null,
  empty: false,
  actions: {
    redraw: function(evt) {
      this.drawGraphs();
    },
    change: function(evt) {
      this.set('data', evt.data.series);
      this.drawGraphs();
      this.rerender();
    },
  },

  drawGraphs: function() {
    if (!this.data) {
      set(this, 'empty', true);
      return;
    }

    let svg = (this.svg = select(this.element.querySelector('svg.sparkline')));
    svg.on('mouseover mousemove mouseout', null);
    svg.selectAll('path').remove();
    svg.selectAll('rect').remove();

    let bb = svg.node().getBoundingClientRect();
    let w = bb.width;
    let h = bb.height;

    // To be safe, filter any series that actually have no data points. This can
    // happen thanks to our current provider contract allowing empty arrays for
    // series data if there is no value.
    let maybeData = this.data || {};
    let series = maybeData.data || [];
    let labels = maybeData.labels || {};
    let unitSuffix = maybeData.unitSuffix || '';
    let keys = Object.keys(labels).filter(l => l != 'Total');

    if (series.length == 0 || keys.length == 0) {
      // Put the graph in an error state that might get fixed if metrics show up
      // on next poll.
      set(this, 'empty', true);
      return;
    } else {
      set(this, 'empty', false);
    }

    let st = stack()
      .keys(keys)
      .order(stackOrderReverse);

    let stackData = st(series);

    // Sum all of the values for each point to get max range. Technically
    // stackData contains this but I didn't find reliable documentation on
    // whether we can rely on the highest stacked area to always be first/last
    // in array etc. so this is simpler.
    let summed = series.map(d => {
      let sum = 0;
      keys.forEach(l => {
        sum = sum + d[l];
      });
      return sum;
    });

    let x = scaleTime()
      .domain(extent(series, d => d.time))
      .range([0, w]);

    let y = scaleLinear()
      .domain([0, max(summed)])
      .range([h, 0]);

    let a = area()
      .x(d => x(d.data.time))
      .y1(d => y(d[0]))
      .y0(d => y(d[1]));

    // Use the grey/red we prefer by default but have more colors available in
    // case user adds extra series with a custom provider.
    let colorScheme = ['#DCE0E6', '#C73445'].concat(schemeTableau10);

    if (keys.includes('Outbound')) {
      colorScheme = ['#DCE0E6', '#0E40A3'].concat(schemeTableau10);
    }
    let color = scaleOrdinal(colorScheme).domain(keys);

    svg
      .selectAll('path')
      .data(stackData)
      .join('path')
      .attr('fill', ({ key }) => color(key))
      .attr('stroke', ({ key }) => color(key))
      .attr('d', a);

    let cursor = svg
      .append('rect')
      .attr('class', 'cursor')
      .style('visibility', 'hidden')
      .attr('width', 1)
      .attr('height', h)
      .attr('x', 0)
      .attr('y', 0);

    let tooltip = select(this.element.querySelector('.tooltip'));
    tooltip.selectAll('.sparkline-tt-legend').remove();
    tooltip.selectAll('.sparkline-tt-sum').remove();

    for (var k of keys) {
      let legend = tooltip.append('div').attr('class', 'sparkline-tt-legend');

      legend
        .append('div')
        .attr('class', 'sparkline-tt-legend-color')
        .style('background-color', color(k));

      legend
        .append('span')
        .text(k)
        .append('span')
        .attr('class', 'sparkline-tt-legend-value');
    }

    let tipVals = tooltip.selectAll('.sparkline-tt-legend-value');

    // Add a label for the summed value
    if (keys.length > 1) {
      tooltip
        .append('div')
        .attr('class', 'sparkline-tt-sum')
        .append('span')
        .text('Total')
        .append('span')
        .attr('class', 'sparkline-tt-sum-value');
    }

    let self = this;
    svg
      .on('mouseover', function(e) {
        tooltip.style('visibility', 'visible');
        cursor.style('visibility', 'visible');
        // We update here since we might redraw the graph with user's cursor
        // stationary over it. If that happens mouseover fires but not
        // mousemove but the tooltip and cursor are wrong (based on old data).
        self.updateTooltip(e, series, stackData, summed, unitSuffix, x, tooltip, tipVals, cursor);
      })
      .on('mousemove', function(e) {
        self.updateTooltip(e, series, stackData, summed, unitSuffix, x, tooltip, tipVals, cursor);
      })
      .on('mouseout', function(e) {
        tooltip.style('visibility', 'hidden');
        cursor.style('visibility', 'hidden');
      });
  },
  willDestroyElement: function() {
    this._super(...arguments);
    if (typeof this.svg !== 'undefined') {
      this.svg.on('mouseover mousemove mouseout', null);
    }
  },
  updateTooltip: function(e, series, stackData, summed, unitSuffix, x, tooltip, tipVals, cursor) {
    let [mouseX] = pointer(e);
    cursor.attr('x', mouseX);

    let mouseTime = x.invert(mouseX);
    var bisectTime = bisector(function(d) {
      return d.time;
    }).left;
    let tipIdx = bisectTime(series, mouseTime);

    tooltip
      // 22 px is the correction to align the arrow on the tool tip with
      // cursor.
      .style('left', mouseX - 22 + 'px')
      .select('.sparkline-time')
      .text(niceTimeWithSeconds(mouseTime));

    // Get the summed value - that's the one of the top most stack.
    tooltip.select('.sparkline-tt-sum-value').text(`${shortNumStr(summed[tipIdx])}${unitSuffix}`);

    tipVals.nodes().forEach((n, i) => {
      let val = stackData[i][tipIdx][1] - stackData[i][tipIdx][0];
      select(n).text(`${shortNumStr(val)}${unitSuffix}`);
    });
    cursor.attr('x', mouseX);
  },
});

// Duplicated in vendor/metrics-providers/prometheus.js since we want that to
// remain a standalone example of a provider that could be loaded externally.
function shortNumStr(n) {
  if (n < 1e3) {
    if (Number.isInteger(n)) return '' + n;
    if (n >= 100) {
      // Go to 3 significant figures but wrap it in Number to avoid scientific
      // notation lie 2.3e+2 for 230.
      return Number(n.toPrecision(3));
    }
    if (n < 1) {
      // Very small numbers show with limited precision to prevent long string
      // of 0.000000.
      return Number(n.toFixed(2));
    } else {
      // Two sig figs is enough below this
      return Number(n.toPrecision(2));
    }
  }
  if (n >= 1e3 && n < 1e6) return +(n / 1e3).toPrecision(3) + 'k';
  if (n >= 1e6 && n < 1e9) return +(n / 1e6).toPrecision(3) + 'm';
  if (n >= 1e9 && n < 1e12) return +(n / 1e9).toPrecision(3) + 'g';
  if (n >= 1e12) return +(n / 1e12).toFixed(0) + 't';
}
