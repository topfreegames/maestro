import falcon
import json

import app

class Healthcheck (object):
    def on_get(self, req, resp):
        """Handles GET requests"""
        app.logger.debug("healthcheck")
        resp.status = falcon.HTTP_200
        resp.body = (json.dumps({"healthy": True}))