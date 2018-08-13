import falcon
import json
import requests

import app
import constant

class Address (object):
    def on_get(self, req, resp):
        """Handles GET requests"""
        app.logger.debug("address")
        app.logger.debug("request to {}/{}".format(constant.MAESTRO_ADDR, constant.ROOM_ADDR_ENDPOINT))

        try:
            r = requests.get("{}/{}".format(constant.MAESTRO_ADDR, constant.ROOM_ADDR_ENDPOINT))
            if r.status_code == 200:
                resp.status = falcon.HTTP_200
                resp.body = r.text

        except Exception as ex:
            resp.status = falcon.HTTP_500
            resp.body = (json.dumps({"error": str(ex)}))