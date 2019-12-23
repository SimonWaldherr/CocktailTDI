#!/bin/bash
env GOOS=linux GOARCH=arm GOARM=5 go build cocktail.go
scp ./cocktail pi@192.168.178.202:/home/pi/bin/cocktail
ssh pi@192.168.178.202 /home/pi/bin/runCocktail.sh
