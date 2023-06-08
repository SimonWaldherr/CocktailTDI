# CocktailTDI_Beta

[![Go Report Card](https://goreportcard.com/badge/github.com/simonwaldherr/cocktailtdi)](https://goreportcard.com/report/github.com/simonwaldherr/cocktailtdi)  

work in progress

## howto

### ad your public key to the raspberry

```sh
ssh-copy-id pi@192.168.178.202
```

### compile

```sh
./buildForRaspi.sh
```

### copy to Raspberry

```sh
scp ./cocktail pi@192.168.178.202:/home/pi/bin/cocktail
```

### run

```sh
./cocktail
```

### reset IO-Pin

```sh
echo 5 > /sys/class/gpio/unexport
```

### fix bugs and add new features

```sh
vi cocktail.go
```

### commit to git

```sh
git commit -m "description"
```

### [repeat](#compile)

