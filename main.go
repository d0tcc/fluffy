package main

import (
	"fmt"
	"log"
	"os"
	"time"
	"gonfig"
	"path"

	"github.com/dhowden/raspicam"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

	gobot "gobot.io/x/gobot"
	"gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/platforms/raspi"
)

/* global variable declaration */
var tBot *tgbotapi.BotAPI
var mBot *gobot.Robot

/* loading configuration file */
type Configuration struct {
	tbotToken 		string
	adminID 		int64
	pirGPIO   		string
	pictureFolder 	string
}
configuration := Configuration{}
err := gonfig.GetConf("config/config.prod.json", &configuration)


func initMotionBot() {
	raspiAdaptor := raspi.NewAdaptor()

	sensor := gpio.NewPIRMotionDriver(raspiAdaptor, configuration.pirGPIO)

	work := func() {
		sensor.On(gpio.MotionDetected, func(data interface{}) {
			fmt.Println(gpio.MotionDetected)
			sendTextAsBot("Motion detected!")
			sendPhotoAsBot()
			time.Sleep(200 * time.Millisecond)
			sendPhotoAsBot()
			time.Sleep(200 * time.Millisecond)
			sendPhotoAsBot()
		})
		sensor.On(gpio.MotionStopped, func(data interface{}) {
			fmt.Println(gpio.MotionStopped)
		})
	}

	mBot = gobot.NewRobot("motionBot",
		[]gobot.Connection{raspiAdaptor},
		[]gobot.Device{sensor},
		work,
	)
}

func initTelegramBot() {
	var err error
	tBot, err = tgbotapi.NewBotAPI(configuration.tbotToken)
	if err != nil {
		log.Panic(err)
	}
	tBot.Debug = true
	log.Printf("Authorized on account %s", tBot.Self.UserName)
}

func takePhoto() (string, error) {
	currentTime := time.Now()
	path := path.Join(configuration.pictureFolder, currentTime.Format("2006-01-02_15:04:05") + ".jpg")
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create file: %v", err)
		return "", err
	}
	defer f.Close()
	s := raspicam.NewStill()
	errCh := make(chan error)
	go func() {
		for x := range errCh {
			fmt.Fprintf(os.Stderr, "%v\n", x)
		}
	}()
	log.Println("Taking photo...")
	raspicam.Capture(s, f, errCh)
	return path, nil
}

func sendPhotoAsBot() {
	imagePath, err := takePhoto()
	if err != nil {
		log.Panic(err)
	} else {
		photo := tgbotapi.NewPhotoUpload(configuration.adminID, imagePath)
		tBot.Send(photo)
	}
}

func sendTextAsBot(text string) {
	msg := tgbotapi.NewMessage(configuration.adminID, text)
	tBot.Send(msg)
}

func activateSurveillance() error {
	go mBot.Start()
	return nil
}

func deactivateSurveillance() error {
	go mBot.Stop()
	return nil
}

func main() {
	initTelegramBot()
	initMotionBot()
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := tBot.GetUpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}
		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		if update.Message.IsCommand() && update.Message.chatID == configuration.adminID {
			switch update.Message.Command() {
			case "help":
				sendTextAsBot("Type /activate, /deactivate or /pic.")
			case "activate":
				err := activateSurveillance()
				if err != nil {
					log.Panic(err)
				} else {
					sendTextAsBot("Surveillance activated.")
				}
			case "deactivate":
				err := deactivateSurveillance()
				if err != nil {
					log.Panic(err)
				} else {
					sendTextAsBot("Surveillance stopped. Welcome home!")
				}
			case "pic":
				sendTextAsBot("Taking photo...")
				sendPhotoAsBot()
			}
		}
	}
}
