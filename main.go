package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JamesClonk/iRdiscordbot/env"
	"github.com/JamesClonk/iRdiscordbot/log"
	"github.com/bwmarrin/discordgo"
)

func main() {
	level := env.Get("LOG_LEVEL", "info")
	token := env.MustGet("BOT_TOKEN")
	log.Infoln("log level:", level)

	// create Discord session using bot token
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Errorf("error creating Discord session: %v", err)
		return
	}

	dg.AddHandler(onMessageCreate)
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// connect
	if err := dg.Open(); err != nil {
		log.Errorf("error opening connection: %v", err)
		return
	}

	log.Infoln("iRdiscordbot is running...")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill, syscall.SIGKILL)
	<-sc

	// disconnect
	dg.Close()
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// ignore messages by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// if m.Content == "!summary" {
	// 	if _, err := s.ChannelMessageSend(m.ChannelID, "https://irvisualizer.jamesclonk.io/season/3492/summary.png?team=TNT%20Racing"); err != nil {
	// 		log.Errorf("error sending message: %v", err)
	// 		return
	// 	}
	// }
	if m.Content == "!summary" {
		embed := discordgo.MessageEmbed{
			Title:       "Radical Racing Challenge - Driver Summary",
			Description: "Shows driver summary data for the whole season",
			Type:        discordgo.EmbedTypeImage,
			Image: &discordgo.MessageEmbedImage{
				URL: fmt.Sprintf("https://irvisualizer.jamesclonk.io/season/3492/summary.png?team=TNT%%20Racing&cb=%d", time.Now().UnixNano()/1000/1000/1000),
			},
		}
		if _, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed); err != nil {
			log.Errorf("error sending message: %v", err)
			return
		}
	}
}
