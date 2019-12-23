#!/bin/bash
cd /home/pi/bin/;
echo 5 > /sys/class/gpio/unexport;
/home/pi/bin/cocktail;
