#!/usr/bin/python3.7
from __future__ import annotations
import encodings
import re
import os
import sys

LOGS_PATH = '/home/cache-nez/.config/simnet/logs'

addr_to_node = {}
content_to_msg = {}


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
        self.finalized = False
        content_to_msg[content] = self

    def add_hop(self, sender: str, receiver: str):
        assert sender not in self.hops, 'duplicate "from" in message {{{}}} hops: from={}, to={} and {}'.format(self.msg, sender, self.hops[sender], receiver)
        self.hops[sender] = receiver

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
                last_addr = self.path[-1].addr
                # message did not reach destination
                if last_addr not in self.hops:
                    self.path.append(DUMMY_NODE)
                    return
                self.path.append(addr_to_node[self.hops[last_addr]])


class BroadcastMessage:
    __messages = {}

    @staticmethod
    def getOrCreate(msg: str, sender: str):
        if msg not in BroadcastMessage.__messages:
            BroadcastMessage(msg, sender)
        return BroadcastMessage.__messages[msg]

    @staticmethod
    def getMessages():
        return BroadcastMessage.__messages.values()

    def __init__(self, msg: str, sender: str):
        assert msg not in BroadcastMessage.__messages
        self.sender = sender
        self.message = msg
        self.hops = set()
        BroadcastMessage.__messages[msg] = self

    def add_hop(self, sender: str, receiver: str):
        assert (sender, receiver) not in self.hops, 'duplicate hop: {} -> {}'.format(sender, receiver)
        self.hops.add((sender, receiver))

    def calculate_path(self, receiver):
        # TODO
        return None

    def finalize(self):
        self.sender = addr_to_node[self.sender]


name_re = re.compile(r'(node[\d*])')
node_address_re = re.compile(r'mino\[([0-9:.]*)\] is running')
relay_re = re.compile(r'relay opened addr=([0-9:.]*) to=([0-9:.]*)')
receive_re = re.compile(r'got \{([^#]*).*} from [Orchestrator:]*([0-9:.]*)')
sending_unicast_re = re.compile(r'sending {([^#]*).*} to \[[Orchestrator:]*([0-9:.]*)\]')
hop_re = re.compile(r'Forwarding \{([^#]*).*\}, previous hop: ([0-9:.]*), source: ([Orchestrator0-9:.]*), destination: \[(.*)\]')
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
        except Exception as e:
            print('Unexpected exception when searching the encoding: {}'.format(e))
            exit(1)
    return None


starting_nondigit_re = re.compile(r'\A(\D)*')
ansi_colors_re = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')


def strip_colors(filename):
    content = []
    enc = guess_encoding(filename)
    assert enc is not None
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


def main():
    for file in os.scandir(LOGS_PATH):
        strip_colors(file.path)
        read_logs(file.path)
    if len(sys.argv) > 1 and sys.argv[1] == '--hops':
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
        return

    for n in sorted(addr_to_node.values(), key=lambda n: n.name):
        # n.finalize()
        if n == DUMMY_NODE:
            continue
        print('{} ({}): {} open connections, {} messages received'.format(
            n.name, n.addr, len(n.connections), len(n.received_msgs)))
    for m in sorted(content_to_msg.values(), key=lambda m: m.msg):
        m.finalize()
        hops = len(m.hops)
        print('message "{}": {} hops: {}'.format(m.msg, hops,
                                                 ' -> '.join(map(lambda n: n.name, m.path))))
    for bm in BroadcastMessage.getMessages():
        # bm.finalize()
        # first hop is always orchestrator-server -> orchestrator-client
        hops = len(bm.hops) - 1
        print('broadcast message "{}": {} hops'.format(bm.message, hops))


if __name__ == '__main__':
    main()