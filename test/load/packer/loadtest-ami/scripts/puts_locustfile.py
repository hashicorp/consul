import random
import json
from locust import HttpUser, task, between
from locust.user.wait_time import constant_pacing


class WriteOpsUser(HttpUser):

    wait_time = constant_pacing(1)

    @task
    def index(self):
        payload = str(random.getrandbits(500))
        base = "/v1/kv/"
        random_key = f'{str(random.random())}/'
        uri = base + random_key
        self.client.put(uri, data=json.dumps(payload))
