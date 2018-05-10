export default function(distance) {
  return function(name, coordinates) {
    var min = 999999999;
    var max = -999999999;
    var distances = [];
    coordinates.forEach(function(node) {
      if (name == node.Node) {
        var segment = node.Segment;
        coordinates.forEach(function(other) {
          if (node.Node != other.Node && other.Segment == segment) {
            var dist = distance(node, other);
            distances.push({ node: other.Node, distance: dist, segment: segment });
            if (dist < min) {
              min = dist;
            }
            if (dist > max) {
              max = dist;
            }
          }
        });
        distances.sort(function(a, b) {
          return a.distance - b.distance;
        });
      }
    });
    var n = distances.length;
    var halfN = Math.floor(n / 2);
    var median;

    if (n > 0) {
      if (n % 2) {
        // odd
        median = distances[halfN].distance;
      } else {
        median = (distances[halfN - 1].distance + distances[halfN].distance) / 2;
      }
    } else {
      median = 0;
      min = 0;
      max = 0;
    }
    return {
      distances: distances,
      n: distances.length,
      min: parseInt(min * 100) / 100,
      median: parseInt(median * 100) / 100,
      max: parseInt(max * 100) / 100,
    };
  };
}
