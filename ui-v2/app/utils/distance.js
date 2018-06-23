export default function(a, b)
{
  a = a.Coord;
  b = b.Coord;
  let sum = 0;
  for (let i = 0; i < a.Vec.length; i++) {
      var diff = a.Vec[i] - b.Vec[i];
      sum += diff * diff;
  }
  let rtt = Math.sqrt(sum) + a.Height + b.Height;
  const adjusted = rtt + a.Adjustment + b.Adjustment;
  if (adjusted > 0.0) {
      rtt = adjusted;
  }
  return Math.round(rtt * 100000.0) / 100.0;
}
