#!/bin/bash
echo "Start: `date +%m-%d-%Y\ %H:%M:%S\ %Z`.\n"

for i in `seq 1 $1`;
do
	echo "\n\n$i/$1\n==========";
	curl -i -X POST http://localhost:5001/api/1/payments/0x396764f15ed1467883A9a5B7D42AcFb788CD1826/0xBbE54C702A529DF85f2D412F8FC4012Ca0684ba3 -H 'Content-Type: application/json' --data-raw '{"amount": 1}'
done

echo "\nEnd: `date +%m-%d-%Y\ %H:%M:%S\ %Z`."
