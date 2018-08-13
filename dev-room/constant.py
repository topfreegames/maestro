import os

# Config env
MAESTRO_ADDR = "http://{}".format(os.environ.get("MAESTRO_HOST_PORT"))
MAESTRO_SCHEDULER_NAME = os.environ.get("MAESTRO_SCHEDULER_NAME")
MAESTRO_ROOM_ID = os.environ.get("MAESTRO_ROOM_ID")
ROOM_MGMT_ENDPOINT = "scheduler/{}/rooms/{}".format(MAESTRO_SCHEDULER_NAME, MAESTRO_ROOM_ID)
ROOM_ADDR_ENDPOINT = "{}/address".format(ROOM_MGMT_ENDPOINT)
ROOM_STATUS_ENDPOINT = "{}/status".format(ROOM_MGMT_ENDPOINT)
POLLING_INTERVAL_IN_SECONDS = int(os.environ.get("POLLING_INTERVAL_IN_SECONDS")) if os.environ.get("POLLING_INTERVAL_IN_SECONDS") else 10