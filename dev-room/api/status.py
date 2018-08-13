import falcon
import json
import requests
import time

import app
import constant
from api.exceptions import WrongStatus

class Status (object):
    def on_put(self, req, resp):
        """Handles PUT requests"""
        app.logger.debug("status")

        try:
            body = req.stream.read()
            status = str.lower(json.loads(body)["status"])
        
            if status not in ["ready", "occupied"]:
                raise WrongStatus()

            app.logger.debug("request to {}/{} with status={}".format(constant.MAESTRO_ADDR, constant.ROOM_STATUS_ENDPOINT, status))
            r = requests.put("{}/{}".format(constant.MAESTRO_ADDR, constant.ROOM_STATUS_ENDPOINT), json={"timestamp": int(time.time()), "status": status})
            if r.status_code == 200:
                resp.status = falcon.HTTP_200
                resp.body = r.text

        except Exception as ex:
            if type(ex) == WrongStatus:
                resp.status = falcon.HTTP_400
            else:
                resp.status = falcon.HTTP_500
            resp.body = (json.dumps({"error": str(ex)}))
