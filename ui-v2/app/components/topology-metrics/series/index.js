import Component from '@ember/component';
import dayjs from 'dayjs';
import Calendar from 'dayjs/plugin/calendar';

import { select, event, mouse } from 'd3-selection';
import { scaleLinear, scaleTime, scaleOrdinal } from 'd3-scale';
import { schemeTableau10 } from 'd3-scale-chromatic';
import { area, stack, stackOrderReverse } from 'd3-shape';
import { max, extent, bisector } from 'd3-array';

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

  actions: {
    redraw: function(evt) {
      this.drawGraphs();
    },
    change: function(evt) {
      this.data = evt.data;
      this.element.querySelector('.sparkline-loader').style.display = 'none';
      this.drawGraphs();
    },
  },

  drawGraphs: function() {
    if (!this.data.series) {
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
    //
    // TODO(banks): switch series provider data to be a single array with series
    // values as properties as we need below to enforce sensible alignment of
    // timestamps and explicit summing expectations.
    let series = ((this.data || {}).series || []).filter(s => s.data.length > 0);

    if (series.length == 0) {
      // Put the graph in an error state that might get fixed if metrics show up
      // on next poll.
      let loader = this.element.querySelector('.sparkline-loader');
      loader.innerHTML = 'No Metrics Available';
      loader.style.display = 'block';
      return;
    }

    // Fill the timestamps for x axis.
    let data = series[0].data.map(d => {
      return { time: d[0] };
    });
    let keys = [];
    // Initialize zeros
    let summed = this.data.series[0].data.map(d => 0);

    for (var i = 0; i < series.length; i++) {
      let s = series[i];
      // Attach the value as a new field to the data grid.
      s.data.map((d, idx) => {
        data[idx][s.label] = d[1];
        summed[idx] += d[1];
      });
      keys.push(s.label);
    }

    let st = stack()
      .keys(keys)
      .order(stackOrderReverse);

    let stackData = st(data);

    let x = scaleTime()
      .domain(extent(data, d => d.time))
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

    for (var k of keys) {
      let legend = tooltip.append('div').attr('class', 'sparkline-tt-legend');

      legend
        .append('div')
        .attr('class', 'sparkline-tt-legend-color')
        .style('background-color', color(k));

      legend
        .append('span')
        .text(k + ': ')
        .append('span')
        .attr('class', 'sparkline-tt-legend-value');
    }

    let tipVals = tooltip.selectAll('.sparkline-tt-legend-value');

    let self = this;
    svg
      .on('mouseover', function() {
        tooltip.style('visibility', 'visible');
        cursor.style('visibility', 'visible');
        // We update here since we might redraw the graph with user's cursor
        // stationary over it. If that happens mouseover fires but not
        // mousemove but the tooltip and cursor are wrong (based on old data).
        self.updateTooltip(event, data, stackData, keys, x, tooltip, tipVals, cursor);
      })
      .on('mousemove', function(d, i) {
        self.updateTooltip(event, data, stackData, keys, x, tooltip, tipVals, cursor);
      })
      .on('mouseout', function() {
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
  updateTooltip: function(event, data, stackData, keys, x, tooltip, tipVals, cursor) {
    let [mouseX] = mouse(event.currentTarget);
    cursor.attr('x', mouseX);

    let mouseTime = x.invert(mouseX);
    var bisectTime = bisector(function(d) {
      return d.time;
    }).left;
    let tipIdx = bisectTime(data, mouseTime);

    tooltip
      // 22 px is the correction to align the arrow on the tool tip with
      // cursor.
      .style('left', mouseX - 22 + 'px')
      .select('.sparkline-time')
      .text(niceTimeWithSeconds(mouseTime));

    tipVals.nodes().forEach((n, i) => {
      let val = stackData[i][tipIdx][1] - stackData[i][tipIdx][0];
      select(n).text(this.formatTooltip(keys[i], val));
    });
    cursor.attr('x', mouseX);
  },

  formatTooltip: function(label, val) {
    switch (label) {
      case 'Data rate received':
      // fallthrough
      case 'Data rate transmitted':
        return dataRateStr(val);
      default:
        return shortNumStr(val);
    }
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

function dataRateStr(n) {
  return shortNumStr(n) + 'bps';
}
