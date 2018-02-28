#!/bin/sh
PORT_80_PID=/tmp/port_80.pid
HA_PID=/tmp/ha.pid
KILL_LOG=/tmp/ha.kill.log
while true;do
	rm $PORT_80_PID
	n=0
	for p in `netstat -lntp |grep ":80 "|grep "/haproxy"|awk '{print $7}'|sort -g -r|awk -F "/" '{ print $1 }'`;do
		echo "$p " >>$PORT_80_PID
		let n=n+1
	done

	if [ $n -gt 2 ];then
		ps -o pid,comm |grep " haproxy"  > $HA_PID
		pid=`grep -f $PORT_80_PID $HA_PID |awk  '{ if(NR==1 ){print $1} }'`
		if [[ "$pid" != "" ]];then
				echo "kill -9 $pid @ `date`" >>$KILL_LOG
				kill -9  $pid
		fi
	fi
	sleep 60
done
