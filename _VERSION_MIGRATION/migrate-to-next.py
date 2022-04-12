import os
import sys
import requests
import yaml
import json
import time
import argparse
from tqdm import tqdm

# global variables
game = ""
backup_folder_absolute_path = ""
maestro_v9_endpoint = ""
maestro_next_endpoint = ""
container_input_env_vars = {}
dry_run = False


# BACKUP
def make_backup(scheduler_name, yaml_config):
    try:
        file_path = os.path.join(
            backup_folder_absolute_path, f'{scheduler_name}.yaml')
        with open(file_path, "w") as f:
            f.write(yaml_config)
    except Exception as e:
        raise e


# MAPPERS
def get_port_range():
    return {
        'start': 40000,
        'end': 41000
    }


def get_forwarders(forwarders):
    return [{
        "name": 'matchmaking',
        "enable": forwarders['grpc']['matchmaking']['enabled'],
        "type": "gRPC",
        "address": 'matchmaker-rpc.matchmaker.svc.cluster.local:80',
        "options": {
            "timeout": '1000',
            "metadata": forwarders['grpc']['matchmaking']['metadata']
        }
    }]


def get_ports(ports):
    port_list = []
    for port in ports:
        port_list.append({
            "name": port['name'],
            "protocol": str(port['protocol']).lower(),
            "port": port['containerPort']
        })
    return port_list


def get_env(env):
    for env_var in container_input_env_vars:
        exists_to_override = [(index, var) for index, var in enumerate(env) if env_var['name'] == var['name']]
        if len(exists_to_override) > 0:
            env[exists_to_override[0][0]] = env_var
        else:
            env.append(env_var)

    return env


def get_containers(containers):
    containers_list = []
    for container in containers:
        containers_list.append({
            'name': container['name'],
            'image': container['image'],
            'imagePullPolicy': container['imagePullPolicy'],
            'command': container['cmd'],
            'ports': get_ports(container['ports']),
            "environment": get_env(container['env']),
            "requests": container['requests'],
            "limits": container['limits']
        })
    return containers_list


def get_spec(config):
    return {
        'terminationGracePeriod': 100,
        'toleration': config['toleration'],
        'affinity': config['affinity'],
        'containers': get_containers(config['containers'])
    }


def convert_v9_config_to_next(config):
    try:
        next_config = {
            'name': config['name'],
            'game': config['game'],
            'spec': get_spec(config),
            'forwarders': get_forwarders(config['forwarders']),
            'portRange': get_port_range(),
            'maxSurge': '20%'
        }
        return next_config
    except Exception as e:
        raise e


# V9 RELATED
def get_scheduler_config(scheduler):
    try:
        r = requests.get(
            f'{maestro_v9_endpoint}/scheduler/{scheduler["name"]}/config')
        if r.status_code == 200:
            config = r.json()

            loaded_config_as_yaml = yaml.load(
                config['yaml'], Loader=yaml.FullLoader)
            scheduler['yaml'] = config['yaml']
            scheduler['config'] = loaded_config_as_yaml
        else:
            raise Exception("err fetching scheduler config =>", r.text)

        return scheduler
    except Exception as e:
        raise e


def set_min_to_zero(scheduler_name):
    success = True
    reason = ""
    try:
        retry = 3
        for i in range(0, retry):
            r = requests.put(f'{maestro_v9_endpoint}/scheduler/{scheduler_name}', data=json.dumps({
                "min": 0
            }))
            if r.status_code == 200:
                success = True
            else:
                success = False
                reason = r.text

        return success, reason
    except Exception as e:
        raise e


def set_replica_amount(scheduler_name, replicas):
    success = True
    reason = ""
    try:
        retry = 3
        for i in range(0, retry):
            r = requests.post(f'{maestro_v9_endpoint}/scheduler/{scheduler_name}', data=json.dumps({
                "replicas": replicas
            }))
            if r.status_code == 200:
                success = True
            else:
                success = False
                reason = r.text

        return success, reason
    except Exception as e:
        raise e


def get_v9_game_schedulers():
    """
    :returns: [{
        'autoscalingDownTriggerUsage': int,
        'autoscalingMin': int,
        'autoscalingUpTriggerUsage': int,
        'game': string,
        'name': string,
        'roomsCreating': int,
        'roomsOccupied': int,
        'roomsReady': int,
        'roomsTerminating': int,
        'state': string
    }]
    """
    schedulers = []
    try:
        r = requests.get(f'{maestro_v9_endpoint}/scheduler?info')
        if r.status_code == 200:
            schedulers = r.json()
            schedulers = list(
                filter(lambda x: x.get('game') == game, schedulers))
        else:
            raise Exception(
                "could not fetch maestro-v9 endpoint. err =>", r.text)

        return schedulers
    except Exception as e:
        raise e


def delete_scheduler_from_v9(scheduler):
    """Call delete scheduler endpoint for scheduler. Also, guarantee that it was deleted within a 30s timeout.

    scheduler: {
        'autoscalingDownTriggerUsage': int,
        'autoscalingMin': int,
        'autoscalingUpTriggerUsage': int,
        'game': string,
        'name': string,
        'roomsCreating': int,
        'roomsOccupied': int,
        'roomsReady': int,
        'roomsTerminating': int,
        'state': string,
        'yaml': string,
        'config': dict,
        'next-config': dict
    }

    :returns: succeed, reason
    """
    def wait_for_scheduler_to_be_deleted():
        """
        :returns: could_wait
        """
        timeout_in_seconds = 30
        for i in range(0, timeout_in_seconds):
            request = requests.get(f'{maestro_v9_endpoint}/scheduler')
            if request.status_code == 200:
                schedulers = request.json()['schedulers']
                schedulers = list(
                    filter(lambda x: x == scheduler['name'], schedulers))
                if len(schedulers) == 0:
                    return True
                else:
                    time.sleep(1)
            else:
                time.sleep(1)

        return False

    succeed = False
    reason = ""
    retry = 3
    for i in range(0, retry):
        r = requests.delete(f'{maestro_v9_endpoint}/scheduler/{scheduler["name"]}')
        if r.status_code == 200:
            succeed = wait_for_scheduler_to_be_deleted()
            reason = ""
            if not succeed:
                reason = "could not wait for scheduler to be deleted"
                continue
            break
        else:
            succeed = False
            reason = r.text
    return succeed, reason


def create_v9_scheduler(scheduler):
    r = requests.post(f'{maestro_v9_endpoint}/scheduler',
                      data=scheduler['yaml'])
    print(f'rollback scheduler to v9. result => {r.text}')


# NEXT RELATED
def create_next_scheduler(scheduler):
    """Calls the post scheduler endpoint (maestro next). Also, waits 30 seconds for the scheduler to be created,

    scheduler: {
        'autoscalingDownTriggerUsage': int,
        'autoscalingMin': int,
        'autoscalingUpTriggerUsage': int,
        'game': string,
        'name': string,
        'roomsCreating': int,
        'roomsOccupied': int,
        'roomsReady': int,
        'roomsTerminating': int,
        'state': string,
        'yaml': string,
        'config': dict,
        'next-config': dict
    }

    :returns: created, reason
    """

    def wait_for_scheduler_to_be_created():
        """
        :returns: created
        """
        timeout_in_seconds = 30
        for i in range(0, timeout_in_seconds):
            request = requests.get(f'{maestro_next_endpoint}/schedulers/{scheduler["name"]}')
            if request.status_code == 200:
                request2 = requests.get(f'{maestro_next_endpoint}/schedulers/{scheduler["name"]}/operations')
                if request2.status_code == 200:
                    finished_operations = request2.json()["finishedOperations"]
                    # if the operation finishes with error, in this case, it's like it never existed
                    if len(finished_operations) > 0:
                        return True

            time.sleep(1)
        return False

    r = requests.post(f'{maestro_next_endpoint}/schedulers',
                      data=json.dumps(scheduler["next-config"]))
    if r.status_code == 200:
        created = wait_for_scheduler_to_be_created()
        if not created:
            return created, "could not wait for scheduler to be created"

        return created, ""
    else:
        return False, r.text


def wait_for_operation_by_id(scheduler_name, operation_id):
    """Waits for 240 sec
    :returns: succeeded, reason
    """
    timeout_in_seconds = 240
    for i in range(0, timeout_in_seconds):
        request = requests.get(f'{maestro_next_endpoint}/schedulers/{scheduler_name}/operations')
        if request.status_code == 200:
            finished_operations = request.json()["finishedOperations"]
            if len(finished_operations) > 1:
                finished = list(filter(lambda x: x["id"] == operation_id, finished_operations))
                if len(finished) > 0:
                    finished_operation = finished[0]
                    if finished_operation['status'] == 'finished':
                        return True, ""
                    else:
                        execution_history = finished_operation["executionHistory"]
                        return False, execution_history

        time.sleep(1)
    return False, "timeout waiting for operation to finish"


def create_rooms_existed_before(scheduler):
    """
    scheduler: {
        'autoscalingDownTriggerUsage': int,
        'autoscalingMin': int,
        'autoscalingUpTriggerUsage': int,
        'game': string,
        'name': string,
        'roomsCreating': int,
        'roomsOccupied': int,
        'roomsReady': int,
        'roomsTerminating': int,
        'state': string,
        'yaml': string,
        'config': dict,
        'next-config': dict
    }

    :returns: created, reason
    """
    r = requests.post(f'{maestro_next_endpoint}/schedulers/{scheduler["name"]}/add-rooms', data=json.dumps({
        'amount': scheduler['roomsReady']
    }))
    if r.status_code == 200:
        operation_id = r.json()['operationId']
        could_create, reason = wait_for_operation_by_id(scheduler["name"], operation_id)
        return could_create, reason
    else:
        return False, r.text


# SCRIPT SPECIFIC METHODS
def map_configs_for_schedulers(schedulers):
    """
    schedulers: [{
        'autoscalingDownTriggerUsage': int,
        'autoscalingMin': int,
        'autoscalingUpTriggerUsage': int,
        'game': string,
        'name': string,
        'roomsCreating': int,
        'roomsOccupied': int,
        'roomsReady': int,
        'roomsTerminating': int,
        'state': string
    }]

    :return: scheduler
    """
    for index, scheduler in enumerate(tqdm(schedulers)):
        schedulers[index] = get_scheduler_config(scheduler)

    return schedulers


def map_maestro_next_configs_for_scheduler(schedulers):
    for index, scheduler in enumerate(tqdm(schedulers)):
        schedulers[index]["next-config"] = convert_v9_config_to_next(
            scheduler["config"])

    return schedulers


def main():
    try:
        if dry_run:
            print('***** DRY RUN *****')

        print(f"=====> finding v9 schedulers for game: '{game}'")
        schedulers = get_v9_game_schedulers()
        if len(schedulers) == 0:
            print(f"=====> no schedulers found to switch for game: '{game}'")
            sys.exit()
        print("...success")

        print(f"=====> mapping configs for schedulers")
        schedulers = map_configs_for_schedulers(schedulers)
        print("...success")

        print("=====> mapping Next format schedulers")
        schedulers = map_maestro_next_configs_for_scheduler(schedulers)
        print("...success")

        if dry_run:
            print('schedulers meant to be migrated:\n', *list(map(lambda x: f"{x.get('name')}\n", schedulers)))
            print(f'converted scheduler example:\n', yaml.dump(schedulers[0]['next-config'], indent=2))
            sys.exit()

        print("##### all set to start migration! #####")
        for scheduler in tqdm(schedulers):
            print(f'.{scheduler.get("name")} - start')
            scheduler_name = scheduler["name"]

            make_backup(scheduler["name"], scheduler['yaml'])
            print(f'.{scheduler.get("name")} - backup done')

            success, reason = set_min_to_zero(scheduler["name"])
            if not success:
                print(f"ERROR: could not set min to 0 to scheduler '{scheduler_name}'. reason=> {reason}")
                print(f"INFO: stop execution")
                sys.exit()
            print(f'.{scheduler.get("name")} - min set to 0')

            success, reason = set_replica_amount(scheduler["name"], 0)
            if not success:
                print(f"ERROR: could not set replicas to 0 to scheduler '{scheduler_name}'. reason=> {reason}")
                print(f"INFO: stop execution")
                sys.exit()
            print(f'.{scheduler.get("name")} - replica set to 0')

            deleted, reason = delete_scheduler_from_v9(scheduler)
            if not deleted:
                print(f"ERROR: could not delete scheduler '{scheduler_name}'. reason=> {reason}")
                print(f"INFO: stop execution")
                sys.exit()
            print(f'.{scheduler.get("name")} - deleted')

            created, reason = create_next_scheduler(scheduler)
            if not created:
                print(
                    f"ERROR: could not create scheduler '{scheduler_name}' on next. reason=> {reason}")
                print(f"INFO: stop execution")
                create_v9_scheduler(scheduler)
                sys.exit()
            print(f'.{scheduler.get("name")} - created on next')

            created, reason = create_rooms_existed_before(scheduler)
            if not created:
                print(f"WARN: could not create rooms for scheduler '{scheduler_name}'. reason => {reason}")
                print(f"INFO: stop execution")
                sys.exit()
            print(f'.{scheduler.get("name")} - new rooms created')

            print(f'.{scheduler.get("name")} - done')
        print("=====> migration finished")
    except Exception as e:
        print('Script execution failed. err =>', e)


def setup():
    global maestro_v9_endpoint
    global maestro_next_endpoint
    global game
    global backup_folder_absolute_path
    global container_input_env_vars
    global dry_run

    my_parser = argparse.ArgumentParser(description='Args necessary to migrate schedulers from v9 to v10')
    my_parser.add_argument('-o',
                           '--old_url',
                           metavar='v9_url',
                           type=str,
                           required=True,
                           help='Maestro v9 API endpoint')

    my_parser.add_argument('-n',
                           '--new_url',
                           metavar='v10_url',
                           type=str,
                           required=True,
                           help='Maestro v10 API endpoint')

    my_parser.add_argument('-b',
                           '--backup',
                           metavar='bkp_folder',
                           type=str,
                           required=True,
                           help='Backup folder to store schedulers yaml files')

    my_parser.add_argument('-g',
                           '--game',
                           metavar='game',
                           type=str,
                           required=True,
                           help='Name of the game containing the schedulers to be migrated')

    my_parser.add_argument('-f',
                           '--yaml_file',
                           metavar='container_env_vars_file',
                           type=str,
                           required=True,
                           help='yaml file containing env variables to be overridden in the format "env: -name: value:"')

    my_parser.add_argument('-d',
                           '--dry',
                           help='dry-run to test the expected execution',
                           action='store_true')

    args = my_parser.parse_args()

    maestro_v9_endpoint = args.old_url
    maestro_next_endpoint = args.new_url
    game = args.game
    backup_folder_absolute_path = args.backup
    dry_run = args.dry

    with open(args.yaml_file, "r") as f:
        yaml_file = yaml.load(f, Loader=yaml.loader.SafeLoader)

    if not os.path.isdir(backup_folder_absolute_path):
        print('The backup path specified does not exist')
        sys.exit()

    env_vars = yaml_file.get('env')
    if not env_vars:
        print('The yaml file do not have "env" property')
        sys.exit()

    for var in env_vars:
        if not var.get('name'):
            print(f'invalid syntax for {var} at yaml file')
            sys.exit()

    container_input_env_vars = env_vars

    v9_last_char = maestro_v9_endpoint[-1]
    next_last_char = maestro_next_endpoint[-1]

    if v9_last_char == '/':
        maestro_v9_endpoint = maestro_v9_endpoint.rstrip(
            maestro_v9_endpoint[-1])
    if next_last_char == '/':
        maestro_next_endpoint = maestro_next_endpoint.rstrip(
            maestro_next_endpoint[-1])


if __name__ == '__main__':
    setup()
    main()
