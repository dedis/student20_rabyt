#!/usr/bin/python3.7
from __future__ import annotations
from collections import namedtuple
import encodings
import re
import os
import sys

LOGS_PATH = '/home/cache-nez/.config/simnet/logs'

addr_to_node = {}
content_to_msg = {}
broadcast_messages = {}


class Node:
    def __init__(self, addr: str, name: str, is_orchestrator: bool):
        self.addr = addr
        self.name = name
        self.orchestrator = is_orchestrator
        self.connections = set()
        self.received_msgs = set()
        # self.finalized = False
        addr_to_node[addr] = self

    def open_connection(self, to: str):
        self.connections.add(to)

    def receive_msg(self, msg: str):
        self.received_msgs.add(msg)

    # called after all logs are read, therefore all nodes and messages created
    def finalize(self):
        if not self.finalized:
            self.connections = set(map(lambda n: addr_to_node[n], self.connections))
            self.received_msgs = set(map(lambda c: content_to_msg[c], self.received_msgs))
            self.finalized = True


DUMMY_NODE = Node("0.0.0.0", "BLOCK", False)


class Message:
    def __init__(self, content: str, sender: str, receiver: str):
        assert content not in content_to_msg, 'duplicate message: ' + content
        self.msg = content
        self.sender = sender
        self.receiver = receiver
        self.hops = {}
        self.path = None
        self.delivered = False
        self.finalized = False
        content_to_msg[content] = self

    def add_hop(self, sender: str, receiver: str):
        if sender in self.hops:
            print('duplicate "from" in message {{{}}} hops: from={}, to={} and {}'.format(self.msg, sender, self.hops[sender], receiver))
        # assert sender not in self.hops, 'duplicate "from" in message {{{}}} hops: from={}, to={} and {}'.format(self.msg, sender, self.hops[sender], receiver)
        self.hops[sender] = receiver
        if receiver == self.receiver:
            self.delivered = True

    def finalize(self):
        if not self.finalized:
            self.sender = addr_to_node[self.sender]
            self.receiver = addr_to_node[self.receiver]
            for addr, node in addr_to_node.items():
                if addr in self.msg:
                    self.msg = self.msg.replace(addr, node.name)
            self.finalized = True
            self.calculate_path()

    def calculate_path(self):
        if self.path is None:
            assert self.finalized
            self.path = [self.sender]
            while self.path[-1].addr != self.receiver.addr:
                # message did not reach destination
                if self.path[-1] == DUMMY_NODE:
                    return
                last_addr = self.path[-1].addr
                self.path.append(addr_to_node[self.hops[last_addr]])


class BroadcastMessage:
    __messages = {}

    @staticmethod
    def getOrCreate(msg: str, sender: str):
        if msg not in broadcast_messages:
            BroadcastMessage(msg, sender)
        return broadcast_messages[msg]

    @staticmethod
    def getMessages():
        return broadcast_messages.values()

    def __init__(self, msg: str, sender: str):
        assert msg not in broadcast_messages
        self.sender = sender
        self.message = msg
        self.hops = set()
        broadcast_messages[msg] = self

    def add_hop(self, sender: str, receiver: str):
        assert (sender, receiver) not in self.hops, 'duplicate hop: {} -> {}'.format(sender, receiver)
        self.hops.add((sender, receiver))

    def calculate_path(self, receiver):
        # TODO
        return None

    def finalize(self):
        self.sender = addr_to_node[self.sender]


DISCONNECT = 'disconnect'
REPLY_ALL = 'replyAll'
PERCENTAGE = 'percentage'
PROTOCOL = 'protocol'
SimulationParams = namedtuple('SimulationParams', [DISCONNECT, REPLY_ALL, PERCENTAGE, PROTOCOL])
# maps a stat to a dict numNodes -> [value_run_1, value_run_2, value_run_3]
Stats = namedtuple('Stats', ['openConnections', 'sent', 'delivered', 'broadcastHops', 'unicastAvgHops'])
# dict params -> stats
stats = {}


def get_defaults():
    return {DISCONNECT: False, REPLY_ALL: False, PERCENTAGE: 0, PROTOCOL: 'prefix'}


protocol_re = re.compile(r'protocol=([a-z]*)')
percentage_re = re.compile(r'disconnect-percentage=([0-9]*)')


def logdir_to_params(dirname):
    params = get_defaults()
    if DISCONNECT in dirname:
        params[DISCONNECT] = True
    if REPLY_ALL in dirname:
        params[REPLY_ALL] = True
    if PERCENTAGE in dirname:
        p = percentage_re.search(dirname).groups()[0]
        params[PERCENTAGE] = int(p)
    if PROTOCOL in dirname:
        params[PROTOCOL] = protocol_re.search(dirname).groups()[0]
    return SimulationParams(**params)


def process_dir(dirname):
    global content_to_msg
    global broadcast_messages
    global addr_to_node
    params = logdir_to_params(dirname)
    if params not in stats:
        stats[params] = Stats({}, {}, {}, {}, {})
    empty = True
    for file in os.scandir(dirname):
        if file.path.endswith('.log'):
            empty = False
            strip_colors(file.path)
            read_logs(file.path)
    if empty:
        return False
    # subtract dummy
    if DUMMY_NODE.addr in addr_to_node:
        del addr_to_node[DUMMY_NODE.addr]
    numNodes = len(addr_to_node)
    connections = sum(len(node.connections) for node in addr_to_node.values())
    # unicast messages only
    sent = len(content_to_msg)
    delivered = sum(1 for msg in content_to_msg.values() if msg.delivered)
    totalHops = sum(len(msg.hops) for msg in content_to_msg.values() if msg.delivered)
    bm = list(broadcast_messages.values())[0]
    stats[params].openConnections.setdefault(numNodes, []).append(connections)
    stats[params].broadcastHops.setdefault(numNodes, []).append(len(bm.hops) - 1)
    stats[params].unicastAvgHops.setdefault(numNodes, []).append(totalHops / delivered)
    stats[params].delivered.setdefault(numNodes, []).append(delivered)
    stats[params].sent.setdefault(numNodes, []).append(sent)

    # reset global state
    content_to_msg = {}
    addr_to_node = {}
    broadcast_messages = {}
    return True


def avg(list):
    return sum(list) / len(list)


def calculate_all_stats(stats_dir):
    for dir in os.scandir(stats_dir):
        if os.path.basename(dir.path).startswith('--protocol'):
            print('processing', os.path.basename(dir.path))
            process_dir(dir.path)
    for params in sorted(stats, key=lambda x: str(x)):
        print(params)
        for numNodes in stats[params].openConnections:
            stats[params].openConnections[numNodes] = avg(stats[params].openConnections[numNodes])
            stats[params].broadcastHops[numNodes] = avg(stats[params].broadcastHops[numNodes])
            stats[params].unicastAvgHops[numNodes] = avg(stats[params].unicastAvgHops[numNodes])
            stats[params].sent[numNodes] = avg(stats[params].sent[numNodes])
            stats[params].delivered[numNodes] = avg(stats[params].delivered[numNodes])
        sortedNums = list(sorted(stats[params].openConnections.keys()))
        print('numNodes: ', sortedNums)
        print('open connections: ', [stats[params].openConnections[n] for n in sortedNums])
        print('avg hops: ', [stats[params].unicastAvgHops[n] for n in sortedNums])
        print('broadcast hops: ', [stats[params].broadcastHops[n] for n in sortedNums])
        print('sent messages: ', [stats[params].sent[n] for n in sortedNums])
        print('delivered messages: ', [stats[params].delivered[n] for n in sortedNums])
        print()



name_re = re.compile(r'(node[\d*])')
node_address_re = re.compile(r'mino\[([0-9:.]*)\] is running')
relay_re = re.compile(r'relay opened addr=([0-9:.]*) to=([0-9:.]*)')
receive_re = re.compile(r'got \{([^#]*).*} from [Orchestrator:]*([0-9:.]*)')
sending_unicast_re = re.compile(r'sending {([^#]*).*} to \[[Orchestrator:]*([0-9:.]*)\]')
hop_re = re.compile(r'Forwarding \{([^#]*).*\}, previous hop: ([0-9:.]*), source: ([Orchestrator0-9:.]*), destination: \[(.*)\]')
failed_unicast_re = re.compile(r'Failed to route {([^#]*).*} to \[[Orchestrator:]*([0-9:.]*)\]')
failed_unicast_src_re = re.compile(r'from=([0-9:.]*)')
ORCHESTRATOR_PREFIX = 'Orchestrator:'


def strip_orchestrator(node: str):
    if node.startswith(ORCHESTRATOR_PREFIX):
        return node[len(ORCHESTRATOR_PREFIX):]
    return node


def read_logs(filename):
    node_addr = None
    # nodeN
    node_name = name_re.search(os.path.basename(filename)).groups()[0]
    this_node = None
    # after the logs have been processed, the encoding is the default utf-8
    for line in open(filename):
        if node_address_re.search(line):
            node_addr = node_address_re.search(line).groups()[0]
        if this_node is None and 'received command on the daemon command=0300' in line:
            this_node = Node(node_addr, node_name, True)
        if this_node is None and 'Forwarding' in line:
            this_node = Node(node_addr, node_name, False)
        # if this_node is None, the relay is opened as a part of a previous
        # phase, e.g. certificate exchange
        if this_node is not None and relay_re.search(line):
            fr, to = relay_re.search(line).groups()
            assert fr == node_addr, 'this node is {}, but the relay.from={}'.format(node_addr, fr)
            this_node.open_connection(to)
        if receive_re.search(line):
            msg, fr = receive_re.search(line).groups()
            this_node.receive_msg(msg)
        if sending_unicast_re.search(line):
            msg, to = sending_unicast_re.search(line).groups()
            if msg not in content_to_msg:
                Message(msg, node_addr, to)
        if hop_re.search(line):
            msg, prev, src, dests = hop_re.search(line).groups()
            dests = dests.split()
            # orchestrator
            # 1) always broadcasts messages
            # 2) is the only one who can broadcast
            if src.startswith(ORCHESTRATOR_PREFIX):
                src = strip_orchestrator(src)
                BroadcastMessage.getOrCreate(msg, src).add_hop(prev, node_addr)
            else:
                assert len(dests) == 1, 'a message from {} to {}'.format(src, dests)
                dest = strip_orchestrator(dests[0])
                if msg not in content_to_msg:
                    Message(msg, src, dest)
                # skip the hop from server to client side of orchestrator
                if prev != node_addr:
                    content_to_msg[msg].add_hop(prev, node_addr)
        if failed_unicast_re.search(line):
            msg, dest = failed_unicast_re.search(line).groups()
            src = failed_unicast_src_re.search(line).groups()[0]
            if msg not in content_to_msg:
                Message(msg, src, dest)
            content_to_msg[msg].add_hop(node_addr, DUMMY_NODE.addr)


def guess_encoding(filename):
    for enc in encodings.aliases.aliases.keys():
        try:
            with open(filename, encoding=enc) as inp:
                # each log line contains the file in which logger is called;
                # all the code is under /rabyt/dela or /rabyt/routing
                if 'rabyt' in inp.readline():
                    # file open successfully and the first line was decoded correctly
                    return enc
        except UnicodeDecodeError:
            continue
        except LookupError:
            continue
        except UnicodeError:
            continue
        except Exception as e:
            print('Unexpected exception when searching the encoding: {}'.format(e))
            exit(1)
    return None


starting_nondigit_re = re.compile(r'\A(\D)*')
ansi_colors_re = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')


def strip_colors(filename):
    content = []
    enc = guess_encoding(filename)
    assert enc is not None, filename
    with open(filename, encoding=enc) as inp:
        for line in inp:
            # remove color sequences
            line = ansi_colors_re.sub('', line)
            # remove weird characters before the date
            line = starting_nondigit_re.sub('', line)
            content.append(line)
    with open(filename, 'w') as out:
        for line in content:
            print(line, end='', file=out)


def print_used_links():
    hops = set()
    for bm in BroadcastMessage.getMessages():
        for (src, dest) in bm.hops:
            # add links only in one direction
            if src != dest and (dest, src) not in hops:
                hops.add((src, dest))
    for m in content_to_msg.values():
        for src, dest in m.hops.items():
            # add links only in one direction
            if src != dest and (dest, src) not in hops:
                hops.add((src, dest))
    for (src, dest) in hops:
        print(addr_to_node[src].name, addr_to_node[dest].name)


def print_full_stats():
    for n in sorted(addr_to_node.values(), key=lambda n: n.name):
        # n.finalize()
        if n == DUMMY_NODE:
            continue
        print('{} ({}): {} open connections, {} messages received'.format(
            n.name, n.addr, len(n.connections), len(n.received_msgs)))
    for m in sorted(content_to_msg.values(), key=lambda m: m.msg):
        m.finalize()
        print('message "{}": {} hops: {}'.format(m.msg, len(m.hops),
                                                 ' -> '.join(map(lambda n: n.name, m.path))))
    for bm in BroadcastMessage.getMessages():
        # bm.finalize()
        # first hop is always orchestrator-server -> orchestrator-client
        hops = len(bm.hops) - 1
        print('broadcast message "{}": {} hops'.format(bm.message, hops))


def main():
    if len(sys.argv) == 1:
        logs_dir = LOGS_PATH
    else:
        stats_dir = sys.argv[1]
        calculate_all_stats(stats_dir)
        return
    for file in os.scandir(logs_dir):
        if file.path.endswith('.log'):
            strip_colors(file.path)
            read_logs(file.path)
    if len(sys.argv) > 1 and sys.argv[1] == '--hops':
        print_used_links()
        return
    print_full_stats()


if __name__ == '__main__':
    main()