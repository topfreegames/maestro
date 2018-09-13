import falcon
import json
import requests
import time
import threading

import app
import constant
from api.exceptions import WrongStatus

memLeaker = []

def CPUIntensiveLoop():
    while True:
        time.sleep(1)

class IncreaseCPUUsage (object):
    def on_put(self, req, resp):
        """Handles PUT requests"""
        app.logger.debug("increase CPU usage for 60s")

        for _ in range(600):
            threading.Thread(target=CPUIntensiveLoop, name="cpu").start()

class IncreaseMemUsage (object):
    def on_put(self, req, resp):
        """Handles PUT requests"""
        app.logger.debug("increase Mem usage")
        
        global memLeaker
        for _ in range(50000000):
            memLeaker.append({"leak": "LEAK", "leaking": "LEAKING"})
