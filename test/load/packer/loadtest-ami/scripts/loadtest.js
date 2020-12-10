import http from 'k6/http';
import { uuidv4 } from "https://jslib.k6.io/k6-utils/1.0.0/index.js";
let data = JSON.parse(open('service.json'));
let check = JSON.parse(open('service-check.json'));



export default function() {

  const key = uuidv4();
  const ipaddress = `http://${__ENV.LB_ENDPOINT}:8500`;
  const kv_uri = '/v1/kv/';
  const value = { data: uuidv4() };
  const kv_address = `${ipaddress + kv_uri + key}`
  
  //Put valid K/V
  let res = http.put(kv_address, JSON.stringify(value));
  if (
    !check(res, {
      'kv status code MUST be 200': (res) => res.status == 200,
    })
  ) {
    fail('kv status code was *not* 200');
  }

  //Register Service
  data["ID"] = key;
  data["Name"] = key;
  const service_uri = '/v1/agent/service/register';
  const service_address = `${ipaddress + service_uri }`
  let res = http.put(service_address, JSON.stringify(data))
  if (
    !check(res, {
      'register service status code MUST be 200': (res) => res.status == 200,
    })
  ) {
    fail('register service status code was *not* 200');
  }

  //Register Check
  check["ServiceID"] = key;
  const check_uri = '/v1/agent/check/register';
  const check_address = `${ipaddress + check_uri }`
  let res = http.put(check_address, JSON.stringify(check))
  if (
    !check(res, {
      'register check status code MUST be 200': (res) => res.status == 200,
    })
  ) {
    fail('register check status code was *not* 200');
  }

}

export let options = {
  // 1 virtual user
  vus: 100,
  // 1 minute
  duration: "15m",
  // 95% of requests must complete below 0.280s
  thresholds: { http_req_duration: ["p(95)<280"] },
};
