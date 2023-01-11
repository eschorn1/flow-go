# DIRECTIONS
# 1. cd into flow-go/singleton (where this file lives)
# 2. run: sudo docker inspect localnet_default > net.json
#    --> this must be rerun each time `sudo make start` is run in integration/localnet
# 3. run python3 -i console.py
#    --> this will drop you into a REPL once a few items have run to collect intel

import json, requests

headers = {'Content-Type: application/json'}
instance_ip = dict()
instance_peer_id = dict()
instance_flow_id = dict()
names = []


def get_info(target_ip):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "whoami"}}
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


def ping(target_ip, peer_id):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "ping-peerid", "peerid": peer_id}}
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


def peer_routing(target_ip, peer_id):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "peer-routing", "peerid": peer_id}}
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


# This exercises a private function on the node side
def private_ping(target_ip, peer_id):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "private-ping", "peerid": peer_id}}
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


def libp2p_createstream(target_ip, peer_id):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "libp2p-createStream", "peerid": peer_id}}
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


def dht_peers(target_ip):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "dump-dht"}}
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


def dht_forcerefresh(target_ip):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "dht-forcerefresh"}}
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


def libp2p_addPeer(target_ip, peerInfo):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "libp2p-addPeer", "peerInfo": peerInfo}}
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


def publish(target_ip, topic, data):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "publish-topic-data", "topic": topic, "data": data}}
    print(json.dumps(payload))
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


def publish_bytes(target_ip, data):
    payload = {'commandName': 'ncc-command', 'data': {"cmd": "publish-bytes", "data": data}}
    print(json.dumps(payload))
    r = requests.post(f"http://{target_ip}:9002/admin/run_command", json.dumps(payload))
    return json.loads(r.text)["output"]


# Load network IP config
with open('net.json') as f:
    net = json.load(f)

# Extract instance identifiers
for instance in net[0]["Containers"].values():
    name = instance["Name"][9:]
    names.append(name)  # ; print(name)
    instance_ip[name] = instance["IPv4Address"][:-3]
    if all([x not in name for x in ["access", "collection", "consensus", "execution", "verification"]]): continue
    info = get_info(instance_ip[name])
    instance_peer_id[name] = info["peer_id"]
    instance_flow_id[name] = info["flow_id"]

a1 = "access_1_1"  # <-------- TKTKTKTK this may have a slightly different name?!?!??
a1_ip = instance_ip[a1]

# Ping everyone access_1_1 knows about
x = dht_peers(a1_ip)
for host in x:
    resp = ping(a1_ip, host)
    print(f"Ping from access_1_1({a1_ip}) to peer_id:{host}: {resp} ", end='')
    if host in instance_peer_id.values():
        print(list(instance_peer_id.keys())[list(instance_peer_id.values()).index(host)])
    else: print()

x = publish(a1_ip, "request-collections/1c6559f31afd9b262035d3b684f074fd0ba00dec2779d53be3e08e11880108fd", "0a127075626c69632d707573682d626c6f636b73222000000000000000000000000000000000000000000000000000000000")
print(x)


xx = publish_bytes(a1_ip, "0a127075626c69632d707573682d626c6f636b73222000000000000000000000000000000000000000000000000000000000")
print("wow", xx)

xxx = private_ping(a1_ip, instance_peer_id["collection_1_1"])
print(xxx)
