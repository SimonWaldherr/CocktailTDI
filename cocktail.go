package main

import (
	"encoding/json"
	"fmt"
	waage "github.com/MichaelS11/go-hx711"
	"github.com/stianeikeland/go-rpio"
	"net/http"
	"path/filepath"
	"runtime"
	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/cachedfile"
	"simonwaldherr.de/go/golibs/file"
	"simonwaldherr.de/go/golibs/gopath"
	"simonwaldherr.de/go/golibs/xmath"
	"simonwaldherr.de/go/gwv"
	"strings"
	"time"
)

type Rezepte []struct {
	Name    string `json:"Name"`
	Zutaten []struct {
		Name  string `json:"Name"`
		Menge int    `json:"Menge"`
	} `json:"Zutaten"`
	Kommentar string `json:"Kommentar"`
	Vorher string `json:"Vorher"`
	Nachher string `json:"Nachher"`
}

var pins map[int]int
var zutaten map[string]int
var rezepte Rezepte

func init() {
	pins = map[int]int{
		1:  2,
		2:  3,
		3:  4,
		4:  17,
		5:  27,
		6:  22,
		7:  10,
		8:  9,
		9:  11,
		10: 5,
		11: 6,
		12: 13,
		13: 19,
		14: 26,
		15: 21,
		16: 20,
	}

	err := rpio.Open()
	if err != nil {
		panic(fmt.Sprint("unable to open gpio", err.Error()))
	}
	
	str, _ := file.Read("./zutaten.json")
	err = json.Unmarshal([]byte(str), &zutaten)

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Zutaten geladen:\n")
	}

	str, _ = file.Read("./rezepte.json")
	err = json.Unmarshal([]byte(str), &rezepte)

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Rezepte geladen:\n")
		fmt.Printf("Ende.\n\n")
	}
	runtime.GOMAXPROCS(4)
}

func scaleDelay(scaleDelta int, timeout time.Duration) {
	hx711, err := waage.NewHx711("GPIO6", "GPIO5")

	if err != nil {
		fmt.Println("NewHx711 error:", err)
		return
	}

	defer hx711.Shutdown()

	err = hx711.Reset()
	if err != nil {
		fmt.Println("Reset error:", err)
		return
	}

	hx711.AdjustZero = 128663
	hx711.AdjustScale = 385.000000

	c1 := make(chan bool, 1)
	go func() {
		var tara = []float64{}

		var data float64
		for i := 0; i < 3; i++ {
			time.Sleep(200 * time.Microsecond)

			data, err := hx711.ReadDataMedian(11)
			if err != nil {
				fmt.Println("ReadDataRaw error:", err)
				continue
			}

			tara = append(tara, data)
		}
		taraAvg := float64(xmath.Round(xmath.Arithmetic(tara)))

		fmt.Printf("new tara set to %v\n", taraAvg)

		for {
			time.Sleep(200 * time.Microsecond)
			data2, err := hx711.ReadDataRaw()

			if err != nil {
				fmt.Println("ReadDataRaw error:", err)
				continue
			}

			data = float64(data2-hx711.AdjustZero) / hx711.AdjustScale
			fmt.Println(xmath.Round(data - taraAvg))
			if int(data-taraAvg) > scaleDelta {
				fmt.Printf("data: %v, taraAvg: %v, xdata: %v, scaleDelta: %v\n", data, taraAvg, data-taraAvg, scaleDelta)
				fmt.Println("voll")
				c1 <- true
				return
			}
		}
	}()

	select {
	case _ = <-c1:
		return
	case <-time.After(timeout):
		return
	}
}

func main() {
	dir := gopath.WD()
	fmt.Println("DIR 1:", gopath.WD())
	fmt.Println("DIR 2:", dir)
	HTTPD := gwv.NewWebServer(8080, 60)

	fmt.Println("opening gpio")

	pumpe := rpio.Pin(pins[16])
	master := rpio.Pin(pins[15])
	entluft := rpio.Pin(pins[14])
	pumpe.Input()
	master.Input()
	entluft.Input()

	err := waage.HostInit()
	if err != nil {
		fmt.Println("HostInit error:", err)
		return
	}

	defer rpio.Close()

	pinCola := rpio.Pin(17)

	HTTPD.URLhandler(
		gwv.URL("^/toggle/?$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			for i := 1; i < 17; i++ {
				pinCola := rpio.Pin(pins[i])
				pinCola.Output()
				time.Sleep(time.Second / 5)
				pinCola.Input()
			}
			return "/", http.StatusFound
		}, gwv.HTML),
		gwv.URL("^/ein/?$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			pinCola.Output()
			return "/", http.StatusFound
		}, gwv.HTML),
		gwv.URL("^/aus/?$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			pinCola.Input()
			return "/", http.StatusFound
		}, gwv.HTML),
		gwv.URL("^/select/?.*$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			pin := strings.Replace(req.RequestURI, "/select/", "", 1)
			pinCola = rpio.Pin(pins[int(as.Int(pin))])
			return "", http.StatusOK
		}, gwv.HTML),
		gwv.URL("^/list/?$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			var ret string

			str, _ := file.Read("./rezepte.json")
			err := json.Unmarshal([]byte(str), &rezepte)

			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("Rezepte geladen:\n")
				fmt.Printf("Ende.\n\n")
			}

			for _, cocktail := range rezepte {
				ctname := strings.Replace(cocktail.Name, " ", "", -1)
				ret += fmt.Sprintf("<a href=\"../ozapftis/%v\">%v</a>\n", ctname, cocktail.Name)
				ret += fmt.Sprintf("<p>%v</p>\n\n", cocktail.Kommentar)
			}
			return ret, http.StatusOK
		}, gwv.HTML),
		gwv.URL("^/test/\\d*/\\d*$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			pumpe.Output()
			fmt.Println("pumpe an")
			entluft.Input()
			fmt.Println("entlüft aus")

			fmt.Println(req.RequestURI)

			testStr := strings.Replace(req.RequestURI, "/test/", "", 1)
			testArr := strings.Split(testStr, "/")
			testPin := rpio.Pin(pins[int(as.Int(testArr[0]))])

			fmt.Println("starting go func")

			go func() {
				time.Sleep(2 * time.Second)
				testPin.Output()
				fmt.Println("pumpe an")
				master.Output()
				entluft.Input()
			}()

			fmt.Println("starting scale")

			scaleDelay(int(as.Int(testArr[1])), 4*time.Minute)

			fmt.Println("scale delay ready")

			master.Input()
			entluft.Output()
			time.Sleep(time.Second * 5)
			testPin.Input()
			fmt.Println("stop pump")

			pumpe.Input()
			return "", http.StatusOK
		}, gwv.HTML),
		gwv.URL("^/ozapftis/?.*$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			wunschCocktail := strings.Replace(req.RequestURI, "/ozapftis/", "", 1)
			pumpe := rpio.Pin(pins[16])

			for _, cocktail := range rezepte {
				if strings.Replace(cocktail.Name, " ", "", -1) == wunschCocktail {
					fmt.Printf("Cocktail: %#v\n", cocktail.Name)
					fmt.Printf("  Zutaten:\n")
					for _, zut := range cocktail.Zutaten {
						fmt.Printf("    %v: %v\n", zut.Name, zut.Menge)
						zutatPin := rpio.Pin(pins[zutaten[zut.Name]])
						pumpe.Output()
						fmt.Println("pumpe an")
						entluft.Input()
						fmt.Println("entlüft aus")

						fmt.Println("starting go func")

						go func() {
							time.Sleep(2 * time.Second)
							zutatPin.Output()
							fmt.Println("pumpe an")
							master.Output()
							entluft.Input()
						}()

						fmt.Println("starting scale")

						scaleDelay(int(as.Int(zut.Menge)), 4*time.Minute)

						fmt.Println("scale delay ready")

						master.Input()
						entluft.Output()
						time.Sleep(time.Second * 5)
						zutatPin.Input()
						fmt.Println("stop pump")
						pumpe.Input()
					}
					fmt.Printf("  Kommentar: %#v\n\n", cocktail.Kommentar)
				}
			}
			fmt.Printf("Ende.\n\n")
			return "/", http.StatusFound
		}, gwv.HTML),
		gwv.URL("^/$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			return as.String(cachedfile.Read(filepath.Join(dir, "index.html"))), http.StatusOK
		}, gwv.HTML),
		gwv.Robots(as.String(cachedfile.Read(filepath.Join(dir, "..", "static", "robots.txt")))),
		gwv.Favicon(filepath.Join(dir, "..", "static", "favicon.ico")),
		gwv.StaticFiles("/", dir),
	)

	HTTPD.Start()
	HTTPD.WG.Wait()
}
