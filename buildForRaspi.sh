#!/bin/bash
go fmt
env GOOS=linux GOARCH=arm GOARM=5 go build cocktail.go
scp cocktail index.html multiplikator.json rezepte.json runCocktail.sh test.html pi@192.168.178.202:/home/pi/bin/
ssh pi@192.168.178.202 /home/pi/bin/runCocktail.sh
