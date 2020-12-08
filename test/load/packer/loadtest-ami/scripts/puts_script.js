import http from 'k6/http';
import { uuidv4 } from "https://jslib.k6.io/k6-utils/1.0.0/index.js";


export default function() {

  const key = uuidv4();
  const ipaddress = `http://${__ENV.LB_ENDPOINT}:8500`;
  const uri = '/v1/kv/';
  const value = { data: uuidv4() };
  const address = `${ipaddress + uri + key}`

  const res = http.put(address, JSON.stringify(value));

  console.log(JSON.parse(res.body));
}

export let options = {
  // 1 virtual user
  vus: 100,
  // 1 minute
  duration: "15m",
  // 95% of requests must complete below 0.280s
  thresholds: { http_req_duration: ["p(95)<280"] },
};
