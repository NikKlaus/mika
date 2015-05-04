package util

import (
	"log"
	"git.totdev.in/totv/mika"
	"strings"
	"git.totdev.in/totv/mika/conf"
)

func Debug(msg ...interface{}) {
	if conf.Config.Debug {
		log.Println(msg...)
	}
}

func CaptureMessage(message ...string) {
	if conf.Config.SentryDSN == "" {
		return
	}
	msg := strings.Join(message, "")
	if msg == "" {
		return
	}
	_, err := mika.RavenClient.CaptureMessage()
	if err != nil {
		log.Println("CaptureMessage: Failed to send message:", err)
	}
}