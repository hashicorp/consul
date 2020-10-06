import Component from '@ember/component';

import { select, event, mouse } from 'd3-selection';
import { scaleLinear, scaleTime, scaleOrdinal } from 'd3-scale';
import { schemeTableau10 } from 'd3-scale-chromatic';
import { area, stack, stackOrderAscending } from 'd3-shape';
import { max, extent, bisector } from "d3-array";

export default Component.extend({
  isLoaded: false,
  actions: {
    change: function(evt) {
      this.isLoaded = true
      this.element.querySelector(".sparkline-loader").style.display = 'none';

      let svg = select(this.element.querySelector("svg.sparkline"))
      svg.selectAll('path').remove();
      svg.selectAll('rect').remove();

      let w = svg.attr('width');
      let h = svg.attr('height');

      // TODO handle no series being returned.

      // Fill the timestamps for x axis.
      let data = evt.data.series[0].data.map(d => {return {time: d[0]}});
      let keys = [];
      // Initialize zeros
      let summed = evt.data.series[0].data.map(d => 0);

      for (var i = 0; i < evt.data.series.length; i++) {
        let series = evt.data.series[i];
        // Attach the value as a new field to the data grid.
        series.data.map((d, idx) => {
          data[idx][series.label] = d[1];
          summed[idx] += d[1];
        })
        keys.push(series.label);
      }

      let st = stack()
        .keys(keys);

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

      let color = scaleOrdinal(schemeTableau10)
        .domain(keys);

      svg.selectAll("path")
        .data(stackData)
        .join("path")
          .attr("fill", ({key}) => color(key))
          .attr("opacity", 0.6)
          .attr("stroke", ({key}) => color(key))
          .attr("stroke-width", 2)
          .attr("d", a);

      let cursor = svg.append('rect')
        .attr('class', 'cursor')
        .attr('visibility', 'hidden')
        .attr('width', 1)
        .attr('height', h)
        .attr('x', 0)
        .attr('y', 0);

      let tooltip = select(this.element.querySelector(".tooltip"));
      tooltip.selectAll('.sparkline-tt-legend').remove();

      for (var k of keys) {
        let legend = tooltip.append('div')
          .attr('class', 'sparkline-tt-legend');

        legend.append('div')
            .attr('class', 'sparkline-tt-legend-color')
            // TODO move this to CSS
            .style('width', '1em')
            .style('height', '1em')
            .style('border-radius', 2)
            .style('margin', '0 10px 0 0')
            .style('display', 'inline-block')
            .style('background-color', color(k));

        legend.append('span')
          .text(k+": ")
          .append('b')
          .attr('class', 'sparkline-tt-legend-value');
      }

      let tipVals = tooltip.selectAll('.sparkline-tt-legend-value');

      var bisectTime = bisector(function(d) { return d.time; }).left;

      svg
        .on("mouseover", function(){
          tooltip.style("visibility", "visible");
          cursor.style("visibility", "visible");
        })
        .on("mousemove", function(d, i){
          let [mouseX] = mouse(event.currentTarget);
          let mouseTime = x.invert(mouseX);
          let tipIdx = bisectTime(data, mouseTime);
          tooltip
            // TODO move non-dynamic parts to CSS
            .style("top", "-100%")
            .style("z-index", 100)
            .style("left",(mouseX)+"px")
            .select(".sparkline-time")
              .text(mouseTime);
            
          tipVals.nodes().forEach((n, i) => {
            let val = stackData[i][tipIdx][1];
            select(n)
              .text(val);
          });
          cursor
            .attr('x', mouseX);
        })
        .on("mouseout", function(){
          tooltip.style("visibility", "hidden");
          cursor.style("visibility", "hidden");
        });
    }
  }
});
