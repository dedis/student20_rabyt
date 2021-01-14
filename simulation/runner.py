from dataclasses import dataclass
import subprocess
from typing import List
from os import listdir, path
import shutil
import time
import datetime


numNodesFlag         = "-n"
protocolFlag         = "--protocol"
replyAllFlag         = "--replyAll"
disconnectBeforeFlag = "--disconnect-before"
disconnectAfterFlag  = "--disconnect-after"
percentageFlag       = "--disconnect-percentage"
usedLinksFlag        = "--drop-used-links"
noCertsFlag = "--no-certs"

@dataclass
class SimulationParams:
    protocol: str
    numNodes: int
    replyAll: bool
    disconnectBefore: bool
    disconnectAfter: bool
    dropUsed: bool
    dropPercentage: int
    noCerts: bool

    def __str__(self) -> str:
        params = '--protocol={} -n={}'.format(self.protocol, self.numNodes)
        if self.replyAll:
            params += replyAllFlag
        if self.disconnectBefore:
            params += disconnectBeforeFlag + percentageFlag + '={}'.format(self.dropPercentage)
        if self.disconnectAfter:
            params += disconnectAfterFlag + percentageFlag + '={}'.format(self.dropPercentage)
            if self.dropUsed:
                params += usedLinksFlag
        if self.noCerts:
            params += noCertsFlag
        return params

    def as_params(self) -> List[str]:
        res = [protocolFlag, self.protocol, numNodesFlag, str(self.numNodes)]
        if self.replyAll:
            res.append(replyAllFlag)
        if self.disconnectBefore:
            res.append(disconnectBeforeFlag)
            res.append(percentageFlag)
            res.append(str(self.dropPercentage))
        if self.disconnectAfter:
            res.append(disconnectAfterFlag)
            res.append(percentageFlag)
            res.append(str(self.dropPercentage))
            if self.dropUsed:
                res.append(usedLinksFlag)
        if self.noCerts:
            res.append(noCertsFlag)
        return res

    def as_filename(self) -> str:
        return self.__str__().replace(' ', '')


defaultTreeSimulationParams = SimulationParams('tree', 5, True, False, True, False, 30, True)
defaultPrefixSimulationParams = SimulationParams('prefix', 5, True, False, True, False, 30, True)


oks = """Creating deployment... ok
Waiting deployment... ok
Fetching pods... ok
Configuring pods... ok
Writing topology to pods... ok
Deploying the router... ok
Waiting for the router... ok
Fetching the certificates... ok
"""


def successful_run(logs):
    with open(logs) as inp:
        content = inp.read()
    for ok in oks.split('\n'):
        if ok not in content:
            return False
    return True


LOG_DIR = '/home/cache-nez/.config/simnet/logs/'


def run_simulation(params: SimulationParams, iteration: int):
    errors = 0
    dest_log_dir = path.join('../../simulation-logs', params.as_filename() + '-{}'.format(iteration))
    res = subprocess.run(['mkdir', '--', dest_log_dir])
    # this simulation was already run
    if res.returncode != 0:
        return
    print('running ./simulation', *params.as_params(), datetime.datetime.now())
    simulation_logs = path.join(dest_log_dir, 'simulation-output.txt')
    out = open(simulation_logs, 'w')
    subprocess.run(['./simulation', *params.as_params()], stdout=out, stderr=subprocess.STDOUT)
    out.close()
    while not successful_run(simulation_logs):
        print('error, repeating')
        with open(path.join(dest_log_dir, 'errors' + str(errors)), 'w') as err, open(simulation_logs) as sim:
            err.write(sim.read())
            errors += 1
        out = open(simulation_logs, 'w')
        subprocess.run(['./simulation', *params.as_params()], stdout=out, stderr=subprocess.STDOUT)
        out.close()
    # save logs from this simulation run
    for filename in listdir(LOG_DIR):
        fullname = path.join(LOG_DIR, filename)
        shutil.copy(fullname, dest_log_dir)


def main():
    nodes = [5, 10, 20, 30, 50, 70, 100]
    sim_start = time.time()
    for n in nodes:
        for i in range(1, 4):
            defaultPrefixSimulationParams.numNodes = n
            start = time.time()
            run_simulation(defaultPrefixSimulationParams, i)
            print('ran prefix simulation for {} nodes, took {}'.format(n, datetime.timedelta(seconds=time.time() - start)))

            defaultTreeSimulationParams.numNodes = n
            start = time.time()
            run_simulation(defaultTreeSimulationParams, i)
            print('ran tree simulation for {} nodes, took {}'.format(n, datetime.timedelta(seconds=time.time() - start)))

    print('\nsimulation took {}'.format(datetime.timedelta(seconds=time.time() - sim_start)))


if __name__ == '__main__':
    main()