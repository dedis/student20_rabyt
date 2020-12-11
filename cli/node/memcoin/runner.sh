#!/bin/bash

NUM_NODES=$1

for i in $(eval echo {1..$NUM_NODES})
do
	echo "Spawning node $i"
	LLVL=trace ./memcoin --config "/tmp/node$i" start --port "200$(($i-1))" > /tmp/node$i.log 2>&1 &
done

sleep 10
echo "Exchanging certs"
for i in $(eval echo {1..$NUM_NODES})
do
	for j in $(eval echo {$i..$NUM_NODES})
	do
		if [[ $i -eq $j ]]; then
			continue
		fi
		TOKEN=$(./memcoin --config /tmp/node$i minogrpc token)
		echo "TOKEN: $TOKEN"
		./memcoin --config /tmp/node$j minogrpc join --address "127.0.0.1:200$(($i-1))" $TOKEN
	done
done

sleep 2

COMMAND="./memcoin --config /tmp/node1 minogrpc stream "
for i in $(eval echo {2..$NUM_NODES})
do
	COMMAND+=" --addresses 'F127.0.0.1:200$(($i-1))'"
done
COMMAND+=" --message Hello"

echo "Please execute the below command"
echo $COMMAND

echo "Observe the logs by doing `tail -f /tmp/node1.log`"
echo "Once you're done, execute `pkill memcoin && rm -rf /tmp/node*`"

