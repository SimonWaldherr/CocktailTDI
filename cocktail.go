package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	hx711 "github.com/SimonWaldherr/hx711go"
	nau7802 "github.com/SimonWaldherr/nau7802"

	"github.com/edsrzf/mmap-go"
	"github.com/stianeikeland/go-rpio"
	"golang.org/x/exp/io/i2c"

	"simonwaldherr.de/go/golibs/as"
	"simonwaldherr.de/go/golibs/bitmask"
	"simonwaldherr.de/go/golibs/cachedfile"
	"simonwaldherr.de/go/golibs/file"
	"simonwaldherr.de/go/golibs/gopath"
	"simonwaldherr.de/go/golibs/xmath"
	"simonwaldherr.de/go/gwv"
)

// Rezepte is a struct for the recipes
type Rezepte []struct {
	Name    string `json:"Name"`
	Zutaten []struct {
		Name  string `json:"Name"`
		Menge int    `json:"Menge"`
	} `json:"Zutaten"`
	Kommentar string `json:"Kommentar"`
	Vorher    string `json:"Vorher"`
	Nachher   string `json:"Nachher"`
}

var zutaten map[string]int
var rezepte Rezepte

var TargetWeight int
var AdjustZero int
var AdjustScale float64
var ScaleType string

var pins map[int]int
var i2cDev1, i2cDev2 *i2c.Device
var bm1, bm2 *bitmask.Bitmask

var mutex sync.Mutex

var nau7802d *nau7802.NAU7802

const (
	TCA9548A_ADDRESS = 0x70 // Basisadresse des TCA9548A
)

type TCA9548A struct {
	Dev *i2c.Device
}

func NewTCA9548A(address int) (*TCA9548A, error) {
	dev, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, address)
	if err != nil {
		return nil, err
	}

	return &TCA9548A{Dev: dev}, nil
}

func (p *TCA9548A) SelectChannel(channel byte) error {
	if channel > 7 {
		return fmt.Errorf("invalid channel: %d", channel)
	}
	return p.Dev.Write([]byte{1 << channel})
}

const (
	I2C_ADDR  = "/dev/i2c-1"
	I2C_ADDR2 = "/dev/i2c-0"
)

func setValve(valve int, status bool) {
	var pin int
	pin = pins[valve]

	fmt.Printf("set valve %d on pin %d to %d\n", valve, pin, status)

	if pin > 7 {
		pin = pin - 6
		bm2.Set(pin, !status)
		mutex.Lock()
		time.Sleep(10 * time.Millisecond)
		err := i2cDev2.Write([]byte{byte(bm2.Int())})
		for err != nil {
			fmt.Println(err)
			time.Sleep(10 * time.Millisecond)
			err = i2cDev2.Write([]byte{byte(bm2.Int())})
			time.Sleep(10 * time.Millisecond)
		}

		mutex.Unlock()
		return
	}

	bm1.Set(pin, !status)

	mutex.Lock()
	time.Sleep(10 * time.Millisecond)
	err := i2cDev1.Write([]byte{byte(bm1.Int())})
	for err != nil {
		fmt.Println(err)
		time.Sleep(10 * time.Millisecond)
		err = i2cDev1.Write([]byte{byte(bm1.Int())})
		time.Sleep(10 * time.Millisecond)
	}
	mutex.Unlock()

	//mutex.Lock()
	//time.Sleep(10 * time.Millisecond)
	//nau7802d, _ = nau7802.Initialize()
	//time.Sleep(10 * time.Millisecond)
	//mutex.Unlock()
}

func setPump(status bool) {
	fmt.Printf("set pump to %d\n", status)
	bm2.Set(0, !status)

	mutex.Lock()
	time.Sleep(10 * time.Millisecond)
	err := i2cDev2.Write([]byte{byte(bm2.Int())})
	for err != nil {
		fmt.Println(err)
		time.Sleep(10 * time.Millisecond)
		err = i2cDev2.Write([]byte{byte(bm2.Int())})
		time.Sleep(10 * time.Millisecond)
	}
	mutex.Unlock()
}

func setMasterValve(status bool) {
	fmt.Printf("set master valve to %d\n", status)
	bm2.Set(1, status)

	mutex.Lock()
	time.Sleep(10 * time.Millisecond)
	err := i2cDev2.Write([]byte{byte(bm2.Int())})
	var i int = 0
	for err != nil {
		fmt.Println(err)
		time.Sleep(50 * time.Millisecond)
		err = i2cDev2.Write([]byte{byte(bm2.Int())})
		time.Sleep(50 * time.Millisecond)
		i++

		if i > 5 {
			break
		}
	}
	mutex.Unlock()
}

func setPowerLED(status bool) {
	fmt.Printf("set power-led to %d\n", status)
	pin := rpio.Pin(13)
	pin.Output() // Output mode
	if status == true {
		pin.High()
	} else {
		pin.Low()
	}
}

func setProgressLED(status bool) {
	fmt.Printf("set progress-led to %d\n", status)
	pin := rpio.Pin(21)
	pin.Output() // Output mode
	if status == true {
		pin.High()
	} else {
		pin.Low()
	}
}

func init() {
	pins = map[int]int{
		1:  0,
		2:  1,
		3:  2,
		4:  3,
		5:  4,
		6:  5,
		7:  8,
		8:  9,
		9:  10,
		10: 11,
		11: 12,
		12: 13,
	}

	var err error

	err = rpio.Open()
	if err != nil {
		panic(fmt.Sprint("unable to open gpio", err.Error()))
	}

	i2cDev1, err = i2c.Open(&i2c.Devfs{Dev: I2C_ADDR2}, 0x20)
	if err != nil {
		panic(err)
	}

	i2cDev2, err = i2c.Open(&i2c.Devfs{Dev: I2C_ADDR2}, 0x21)
	if err != nil {
		panic(err)
	}

	bm1 = bitmask.New(0b11111111)
	bm2 = bitmask.New(0b11111111)

	mutex.Lock()
	time.Sleep(10 * time.Millisecond)
	i2cDev1.Write([]byte{byte(bm1.Int())})
	i2cDev2.Write([]byte{byte(bm2.Int())})
	time.Sleep(10 * time.Millisecond)
	mutex.Unlock()

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
	if ScaleType == "hx711" {
		runtime.GC()
		hx711, err := hx711.NewHx711("6", "5")

		if err != nil {
			fmt.Println("NewHx711 error:", err)
			return
		}

		defer hx711.Shutdown()

		for {
			err = hx711.Reset()
			if err == nil {
				break
			}
			log.Print("hx711 BackgroundReadMovingAvgs Reset error:", err)
			time.Sleep(time.Second)
		}

		hx711.AdjustZero = AdjustZero
		hx711.AdjustScale = AdjustScale

		c1 := make(chan bool, 1)
		go func() {
			var tara = []float64{}
			var i int
			var data, predata float64

			fmt.Println("Tara")

			for i = 0; i < 5; i++ {
				time.Sleep(600 * time.Microsecond)

				data, err := hx711.ReadDataMedian(3)
				if err != nil {
					fmt.Println("ReadDataRaw error:", err)
					continue
				}

				tara = append(tara, data)
			}
			taraAvg := float64(xmath.Round(xmath.Arithmetic(tara)))

			fmt.Printf("New tara set to: %v\n", taraAvg)

			for {
				time.Sleep(10 * time.Millisecond)
				data2, err := hx711.ReadDataRaw()

				if err != nil {
					if fmt.Sprintln("ReadDataRaw error:", err) != "ReadDataRaw error: waitForDataReady error: timeout\n" {
						fmt.Println("ReadDataRaw error:", err)
					}
					continue
				}

				predata = data
				data = (float64(data2-hx711.AdjustZero) / hx711.AdjustScale) - taraAvg

				if int(data) > scaleDelta && int(predata) > scaleDelta {
					fmt.Printf("set weight reached. weight is: %d\n", xmath.Round(data))
					c1 <- true
					return
				}
			}
		}()

		select {
		case _ = <-c1:
			return
		case <-time.After(timeout):
			fmt.Println("timeout")
			return
		}
	} else if ScaleType == "nau7802py" {
		// Open the shared memory file
		file, err := os.OpenFile("my_shared_memory", os.O_RDONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// Map the shared memory into the Go process
		smdata, err := mmap.Map(file, mmap.RDONLY, 0)
		if err != nil {
			log.Fatal(err)
		}
		defer smdata.Unmap()

		c1 := make(chan bool, 1)
		go func() {
			var tara = []float32{}
			var i int
			var data, predata float32

			fmt.Println("Tara")

			for i = 0; i < 5; i++ {
				time.Sleep(600 * time.Microsecond)

				data := *(*float32)(unsafe.Pointer(&smdata[0])) * 1000
				fmt.Printf("data sm: %v\n", data)

				tara = append(tara, data)
			}
			taraAvg := float32(xmath.Round(xmath.Arithmetic(tara)))

			fmt.Printf("New tara set to: %v\n", taraAvg)

			for {
				time.Sleep(10 * time.Millisecond)
				data2 := *(*float32)(unsafe.Pointer(&smdata[0])) * 1000
				fmt.Printf("data2 sm: %v, delta: %v\n", data2, scaleDelta)

				if err != nil {
					if fmt.Sprintln("ReadDataRaw error:", err) != "ReadDataRaw error: waitForDataReady error: timeout\n" {
						fmt.Println("ReadDataRaw error:", err)
					}
					continue
				}

				predata = data
				data = data2 - taraAvg

				if int(data) > scaleDelta && int(predata) > scaleDelta {
					fmt.Printf("set weight reached. weight is: %d\n", xmath.Round(float64(data)))
					c1 <- true
					return
				}
			}
		}()

		select {
		case _ = <-c1:
			return
		case <-time.After(timeout):
			fmt.Println("timeout")
			return
		}

		/*
			for {
				// Read data from the shared memory
				value := *(*float32)(unsafe.Pointer(&data[0]))

				// Print the value read from shared memory
				fmt.Println(value)
				time.Sleep(500 * time.Millisecond)
			}
		*/
	} else if ScaleType == "nau7802" {
		c1 := make(chan bool, 1)
		go func() {
			var tara = []float64{}
			var i int
			var data, predata float64

			fmt.Println("Tara")

			var taraAvg float64 = -200000

			for taraAvg < -100000 {
				mutex.Lock()
				time.Sleep(10 * time.Millisecond)
				//nau7802d, _ = nau7802.Initialize()

				i2c_multiplexer, _ := NewTCA9548A(0x70)
				
				for i2 := 0; i2 < 7; i2++ {
					err := i2c_multiplexer.SelectChannel(byte(i2))
					
					if err != nil {
						fmt.Println(err)
					}
					
					i2cDevice, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x20)
					if err != nil {
						panic(err)
					}
					defer i2cDevice.Close()
					
					bitmask := bitmask.New(0b11111111)
					
					i2cDevice.Write([]byte(strconv.Itoa(bitmask.Int())))
					time.Sleep(1500 * time.Millisecond)
					
					for {
						for i := 0; i < 8; i++ {
							bitmask.Set(i, false)
							i2cDevice.Write([]byte{byte(bitmask.Int())})
						}
						time.Sleep(1500 * time.Millisecond)
						for i := 0; i < 8; i++ {
							bitmask.Set(i, true)
							i2cDevice.Write([]byte{byte(bitmask.Int())})
						}
						time.Sleep(1500 * time.Millisecond)
					}
				}
				
				

				dev, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x2A)
				if err != nil {
					fmt.Println(err)
				}

				nau7802d, _ = nau7802.InitializeWithConnection(dev)

				time.Sleep(10 * time.Millisecond)
				mutex.Unlock()
				time.Sleep(20 * time.Millisecond)
				for i = 0; i < 5; i++ {
					time.Sleep(600 * time.Microsecond)

					mutex.Lock()
					time.Sleep(30 * time.Millisecond)
					data, err := nau7802d.GetWeight(true, 3)
					time.Sleep(30 * time.Millisecond)
					mutex.Unlock()

					if err != nil {
						fmt.Println("ReadDataRaw error:", err)
						continue
					}

					if data < -100000 {
						continue
					}

					tara = append(tara, data)
				}
				taraAvg = float64(xmath.Round(xmath.Arithmetic(tara)))
			}

			fmt.Printf("New tara set to: %v\n", taraAvg)

			for {
				time.Sleep(20 * time.Millisecond)

				mutex.Lock()
				time.Sleep(30 * time.Millisecond)
				data2, err := nau7802d.GetWeight(true, 3)
				time.Sleep(30 * time.Millisecond)
				mutex.Unlock()

				if err != nil {
					if fmt.Sprintln("ReadDataRaw error:", err) != "ReadDataRaw error: waitForDataReady error: timeout\n" {
						fmt.Println("ReadDataRaw error:", err)
					}
					continue
				}

				if data2 < -100000 {
					mutex.Lock()
					time.Sleep(10 * time.Millisecond)
					nau7802d, _ = nau7802.Initialize()
					time.Sleep(10 * time.Millisecond)
					mutex.Unlock()
					continue
				}

				predata = data
				//data = (float64(data2-hx711.AdjustZero) / hx711.AdjustScale) - taraAvg
				data = data2 - taraAvg

				if int(data) > scaleDelta && int(predata) > scaleDelta {
					fmt.Printf("set weight reached. weight is: %d\n", xmath.Round(data))
					c1 <- true
					return
				}
			}
		}()

		select {
		case _ = <-c1:
			return
		case <-time.After(timeout):
			fmt.Println("timeout")
			return
		}
	}

}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	runtime.GOMAXPROCS(3)
	flag.IntVar(&TargetWeight, "target", 100, "weight to be measured")
	flag.IntVar(&AdjustZero, "zero", -94932, "adjust zero value")
	flag.Float64Var(&AdjustScale, "scale", 62.8, "adjust scale value")
	flag.StringVar(&ScaleType, "scaleType", "nau7802", "use hx711, nau7802 or nau7802py scale")
	flag.Parse()

	var nau_zero float64

	if ScaleType == "hx711" {
		err := hx711.HostInit()
		if err != nil {
			fmt.Println("HostInit error:", err)
			return
		}
	} else if ScaleType == "nau7802py" {
		// do nothing
	} else if ScaleType == "nau7802" {
		var err error
		mutex.Lock()
		time.Sleep(10 * time.Millisecond)
		
		
		i2c_multiplexer, _ := NewTCA9548A(0x70)
		
		/*
		for i2 := 0; i2 < 7; i2++ {
			err := i2c_multiplexer.SelectChannel(byte(i2))
			
			if err != nil {
				fmt.Println(err)
			}
			
			i2cDevice, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x20)
			if err != nil {
				panic(err)
			}
			defer i2cDevice.Close()
			
			bitmask := bitmask.New(0b11111111)
			
			i2cDevice.Write([]byte(strconv.Itoa(bitmask.Int())))
			time.Sleep(1500 * time.Millisecond)
			
			for {
				for i := 0; i < 8; i++ {
					bitmask.Set(i, false)
					i2cDevice.Write([]byte{byte(bitmask.Int())})
				}
				time.Sleep(1500 * time.Millisecond)
				for i := 0; i < 8; i++ {
					bitmask.Set(i, true)
					i2cDevice.Write([]byte{byte(bitmask.Int())})
				}
				time.Sleep(1500 * time.Millisecond)
			}
		}
		
		*/
		
		err = i2c_multiplexer.SelectChannel(0)
		
		dev, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x2A)
		if err != nil {
			fmt.Println(err)
		}
		
		nau7802d, _ = nau7802.InitializeWithConnection(dev)
		
		
		//nau7802d, err = nau7802.Initialize()
		nau_zero, _ := nau7802d.GetWeight(true, 3)
		fmt.Println(nau_zero)
		time.Sleep(10 * time.Millisecond)
		mutex.Unlock()
		if err != nil {
			fmt.Println("nau7802.Initialize error:", err)
		}
	}

	go func() {
		for {
			weight, err := nau7802d.GetWeight(true, 3)
			if err != nil {
				fmt.Println("## nau7802.GetWeight error:", err)
			} else {
				fmt.Println("## nau7802.GetWeight:", weight-nau_zero)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	//dir := gopath.WD()
	dir := "/home/pi/cocktail/ui/www/"
	fmt.Println("DIR 1:", gopath.WD())
	fmt.Println("DIR 2:", dir)
	HTTPD := gwv.NewWebServer(8081, 60)

	/*
		fmt.Println("opening gpio")

		pumpe := rpio.Pin(pins[16])
		master := rpio.Pin(pins[15])
		entluft := rpio.Pin(pins[14])
		pumpe.Input()
		master.Input()
		entluft.Input()
	*/

	//	defer rpio.Close()

	/*
		pinZutat := rpio.Pin(17)

		for i := 1; i < 15; i++ {
			pinZutat := rpio.Pin(pins[i])
			pinZutat.Output()
			time.Sleep(190 * time.Millisecond)
			pinZutat.Input()
			time.Sleep(190 * time.Millisecond)
		}
	*/
	rand.Seed(time.Now().Unix())

	setPowerLED(true)

	HTTPD.URLhandler(
		/*
			gwv.URL("^/toggle/?$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
				for i := 1; i < 17; i++ {
					pinZutat := rpio.Pin(pins[i])
					pinZutat.Output()
					time.Sleep(time.Second / 5)
					pinZutat.Input()
				}
				return "/", http.StatusFound
			}, gwv.HTML),
			gwv.URL("^/ein/?$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
				pinZutat.Output()
				return "/", http.StatusFound
			}, gwv.HTML),
			gwv.URL("^/aus/?$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
				pinZutat.Input()
				return "/", http.StatusFound
			}, gwv.HTML),
			gwv.URL("^/select/?.*$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
				pin := strings.Replace(req.RequestURI, "/select/", "", 1)
				pinZutat = rpio.Pin(pins[int(as.Int(pin))])
				return "", http.StatusOK
			}, gwv.HTML),
		*/
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
		/*
			gwv.URL("^/test/\\d* /\\d*$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
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
		*/
		gwv.URL("^/ozapftis/?.*$", func(rw http.ResponseWriter, req *http.Request) (string, int) {
			wunschCocktail := strings.Replace(req.RequestURI, "/ozapftis/", "", 1)
			setProgressLED(true)

			var rezept struct {
				Name    string "json:\"Name\""
				Zutaten []struct {
					Name  string "json:\"Name\""
					Menge int    "json:\"Menge\""
				} "json:\"Zutaten\""
				Kommentar string "json:\"Kommentar\""
				Vorher    string "json:\"Vorher\""
				Nachher   string "json:\"Nachher\""
			}

			if wunschCocktail == "random" {
				rezept = rezepte[rand.Intn(len(rezepte))]
			} else {
				for _, cocktail := range rezepte {
					if strings.Replace(cocktail.Name, " ", "", -1) == wunschCocktail {
						rezept = cocktail
					}
				}
			}
			fmt.Printf("Cocktail: %#v\n", rezept.Name)
			fmt.Printf("  Zutaten:\n")

			var vorherigeZutat int = 0

			for _, zut := range rezept.Zutaten {
				//setPump(false)
				//setMasterValve(false)
				setValve(vorherigeZutat, false)

				fmt.Printf("    %v: %v\n", zut.Name, zut.Menge)
				zutatPin := pins[zutaten[zut.Name]]

				fmt.Println("starting go func")

				//go func() {
				time.Sleep(1800 * time.Millisecond)
				setPump(true)
				//setMasterValve(false)
				time.Sleep(2 * time.Second)
				//time.Sleep(6 * time.Second)
				setMasterValve(true)
				time.Sleep(100 * time.Millisecond)
				setValve(zutatPin, true)
				//}()

				scaleDelay(int(as.Int(zut.Menge)), 2*time.Minute)

				setMasterValve(false)
				time.Sleep(10 * time.Millisecond)
				setPump(false)
				time.Sleep(1000 * time.Millisecond)
				setPump(false)

				time.Sleep(2 * time.Second)
				//setValve(zutatPin, false)
				time.Sleep(10 * time.Millisecond)
				setPump(false)

				vorherigeZutat = zutatPin

				time.Sleep(time.Second * 2)

			}

			time.Sleep(10 * time.Millisecond)
			setPump(false)
			setMasterValve(false)
			setValve(vorherigeZutat, false)

			fmt.Printf("  Kommentar: %#v\n\n", rezept.Kommentar)

			fmt.Printf("Ende.\n\n")
			setProgressLED(false)
			return "/", http.StatusFound
		}, gwv.HTML),
		gwv.URL("^/notaus/?.*$", func(rw http.ResponseWriter, req *http.Request) (string, int) {

			setPump(false)
			setMasterValve(false)

			for pin := range pins {
				time.Sleep(time.Millisecond * 20)
				setValve(pin, false)
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
		//gwv.StaticFiles("/ui/", dir),
	)

	HTTPD.Start()
	HTTPD.WG.Wait()
}
