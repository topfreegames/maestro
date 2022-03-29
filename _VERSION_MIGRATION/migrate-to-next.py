import os
import sys
import requests
import yaml
import json
import time
import argparse
from tqdm import tqdm

# Env Vars
game = ""
backup_folder_absolute_path = ""
maestro_v9_endpoint = ""
maestro_next_endpoint = ""


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
def get_port_range(port_range):
    if not port_range:
        return {
            'start': 10000,
            'end': 20000
        }
    else:
        return port_range


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


def get_containers(containers):
    containers_list = []
    for container in containers:
        containers_list.append({
            'name': container['name'],
            'image': container['image'],
            'imagePullPolicy': container['imagePullPolicy'],
            'command': container['cmd'],
            'ports': get_ports(container['ports']),
            "environment": container['env'],
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
            'portRange': get_port_range(config['portRange']),
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

    r = requests.delete(f'{maestro_v9_endpoint}/scheduler/{scheduler["name"]}')
    if r.status_code == 200:
        succeeded = wait_for_scheduler_to_be_deleted()
        if not succeeded:
            return succeeded, f"could not wait for scheduler to be deleted"

        return succeeded, ""
    else:
        return False, r.text


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
        :returns: could_wait
        """
        timeout_in_seconds = 10
        for i in range(0, timeout_in_seconds):
            # TODO: list operations for scheduler, see if operation is finished
            request = requests.get(
                f'{maestro_next_endpoint}/schedulers/{scheduler["name"]}')
            if request.status_code == 200:
                return True
            else:
                time.sleep(1)
                continue
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
        return True, ""
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
        print(f"=====> finding v9 schedulers for game: '{game}'")
        schedulers = get_v9_game_schedulers()
        if len(schedulers) == 0:
            print(f"=====> no schedulers found to switch for game: '{game}'")
            sys.exit()
        print("...success")

        print(f"=====> mapping configs for schedulers")
        schedulers = map_configs_for_schedulers(schedulers)
        time.sleep(1)
        print("...success")

        print("=====> mapping Next format schedulers")
        schedulers = map_maestro_next_configs_for_scheduler(schedulers)
        time.sleep(1)
        print("...success")

        print("##### all set to start migration! #####")
        for scheduler in tqdm(schedulers):
            scheduler_name = scheduler["name"]
            make_backup(scheduler["name"], scheduler['yaml'])

            deleted, reason = delete_scheduler_from_v9(scheduler)
            if not deleted:
                print(
                    f"ERROR: could not delete scheduler '{scheduler_name}'. reason=> {reason}")
                break

            created, reason = create_next_scheduler(scheduler)
            if not created:
                print(
                    f"ERROR: could not create scheduler '{scheduler_name}' on next. reason=> {reason}")
                print(f"INFO: stop execution")
                create_v9_scheduler(scheduler)
                break

            created, reason = create_rooms_existed_before(scheduler)
            if not created:
                print(
                    f"WARN: could not create rooms for scheduler '{scheduler_name}'. reason => {reason}")

        print("=====> migration finished")
    except Exception as e:
        print('Script execution failed. err =>', e)


def setup():
    global maestro_v9_endpoint
    global maestro_next_endpoint
    global game
    global backup_folder_absolute_path

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

    args = my_parser.parse_args()

    maestro_v9_endpoint = args.old_url
    maestro_next_endpoint = args.new_url
    game = args.game
    backup_folder_absolute_path = args.backup

    if not os.path.isdir(backup_folder_absolute_path):
        print('The backup path specified does not exist')
        sys.exit()

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
