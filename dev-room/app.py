# External deps
import time
import requests
import falcon
import logging
from logging.config import fileConfig

# Internal deps 
import constant
from api import Healthcheck, Address, Status


fileConfig('logging_config.ini')
logger = logging.getLogger()

while True:
    try:
        print("polling {}/{}".format(constant.MAESTRO_ADDR, constant.ROOM_ADDR_ENDPOINT))
        r = requests.get("{}/{}".format(constant.MAESTRO_ADDR, constant.ROOM_ADDR_ENDPOINT))
        if r.status_code == 200:
            r = requests.put("{}/{}".format(constant.MAESTRO_ADDR, constant.ROOM_STATUS_ENDPOINT), json={"timestamp": int(time.time()), "status": "ready"})
            break
    except Exception as ex:
        print(str(ex))
        pass
    time.sleep(constant.POLLING_INTERVAL_IN_SECONDS)

# Start falcon API
app = falcon.API()
app.add_route('/healthcheck', Healthcheck())
app.add_route('/address', Address())
app.add_route('/status', Status())