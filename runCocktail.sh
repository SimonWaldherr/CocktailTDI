#!/bin/bash
cd /home/pi/bin/;
echo 5 > /sys/class/gpio/unexport;
#/home/pi/bin/cocktail;
#ssh pi@192.168.178.202
/etc/init.d/ctdiinitd start