#!/bin/bash
cd /home/pi/bin/;
echo 5 > /sys/class/gpio/unexport;
/home/pi/cocktail/cocktail -scaleType=nau7802 &
#sleep 1
#python3.9 readNau7802.py
##/etc/init.d/ctdiinitd start