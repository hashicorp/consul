export default function(scenario, api) {
  // TODO: Abstract this away from HTTP
  scenario
    .given(['the url "$url" responds with a $status status'], function(url, status) {
      return api.server.respondWithStatus(url.split('?')[0], parseInt(status));
    })
    .given(['the url "$url" responds with from yaml\n$yaml'], function(url, data) {
      api.server.respondWith(url.split('?')[0], data);
    })
    .given('a network latency of $number', function(number) {
      api.server.setCookie('CONSUL_LATENCY', number);
    });
}
