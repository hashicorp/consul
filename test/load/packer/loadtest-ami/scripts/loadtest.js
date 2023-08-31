import http from 'k6/http';
import { uuidv4 } from "https://jslib.k6.io/k6-utils/1.0.0/index.js";
import { check, fail } from 'k6';

let data = JSON.parse(open('service.json'));


export default function() {

  const key = uuidv4();
  const ipaddress = `http://${__ENV.LB_ENDPOINT}:8500`;
  const kv_uri = '/v1/kv/';
  const value = { data: uuidv4() };
  const kv_address = `${ipaddress + kv_uri + key}`
  
  //Put valid K/V
  let kvres = http.put(kv_address, JSON.stringify(value));
  if (
    !check(kvres, {
      'kv status code MUST be 200': (kvres) => kvres.status == 200,
    })
  ) {
    fail(`registry check status code was *not* 200. error: ${kvres.error}. body: ${kvres.body}`)
  }

  //Register Service
  data["Name"] = key;
  const service_uri = '/v1/agent/service/register';

  const service_address = `${ipaddress + service_uri }`
  let servres = http.put(service_address, JSON.stringify(data));
  if (
    !check(servres, {
      'register service status code MUST be 200': (servres) => servres.status == 200,
    })
  ) {
    fail(`registry check status code was *not* 200. error: ${servres.error}. body: ${servres.body}`)
  }
}

export let options = {
  // 25 virtual users
  vus: 25,
  // 10 minute
  duration: "10m",
  // 95% of requests must complete below 2.5s
  thresholds: { http_req_duration: ["p(95)<2500"] },
};
