package main

import (
	"github.com/JamesClonk/iRdiscordbot/env"
	"github.com/JamesClonk/iRdiscordbot/log"
)

func main() {
	port := env.Get("PORT", "8080")
	level := env.Get("LOG_LEVEL", "info")
	username := env.MustGet("AUTH_USERNAME")
	//password := env.MustGet("AUTH_PASSWORD")

	log.Infoln("port:", port)
	log.Infoln("log level:", level)
	log.Infoln("auth username:", username)
}
