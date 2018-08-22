# External deps
import time
import requests
import falcon
import logging
import threading
from logging.config import fileConfig

# Internal deps 
import constant
from api import Healthcheck, Address, Status

fileConfig('logging_config.ini')
logger = logging.getLogger()

status = "ready"

def ping():
    while True:
        try:
            print("ping {}/{}".format(constant.MAESTRO_ADDR, constant.ROOM_PING_ENDPOINT))
            print("status {}".format(status))
            requests.put("{}/{}".format(constant.MAESTRO_ADDR, constant.ROOM_PING_ENDPOINT), json={"timestamp": int(time.time()), "status": status})
        except Exception as ex:
            print(str(ex))
            pass
        time.sleep(constant.PING_INTERVAL_IN_SECONDS)


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

threading.Thread(target=ping).start()

# Start falcon API
app = falcon.API()
app.add_route('/healthcheck', Healthcheck())
app.add_route('/address', Address())
app.add_route('/status', Status())