package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JamesClonk/iRdiscordbot/env"
	"github.com/JamesClonk/iRdiscordbot/log"
	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/mux"
)

var (
	rx = regexp.MustCompile("[^0-9]+")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	port := env.Get("PORT", "8080")
	level := env.Get("LOG_LEVEL", "info")
	token := env.MustGet("BOT_TOKEN")

	log.Infoln("port:", port)
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

	// start health endpoint
	go func(s *discordgo.Session) {
		router := mux.NewRouter()
		router.PathPrefix("/health").HandlerFunc(checkHealth(s))

		log.Fatalln(http.ListenAndServe(":"+port, router))
	}(dg)

	log.Infoln("iRdiscordbot is running...")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill, syscall.SIGKILL)
	<-sc

	// disconnect
	dg.Close()
}

func checkHealth(s *discordgo.Session) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		if s.HeartbeatLatency() > time.Duration(time.Minute*5) {
			rw.WriteHeader(500)
			_, _ = rw.Write([]byte(`{ "status": "failed" }`))
			return
		}
		rw.WriteHeader(200)
		_, _ = rw.Write([]byte(`{ "status": "ok" }`))
	}
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// ignore messages by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// easter egg, dutch jokes
	if strings.HasPrefix(strings.ToLower(m.Content), "!martijn") ||
		strings.HasPrefix(strings.ToLower(m.Content), "!anne") ||
		strings.HasPrefix(strings.ToLower(m.Content), "!erwin") ||
		strings.HasPrefix(strings.ToLower(m.Content), "!dutch") ||
		strings.HasPrefix(strings.ToLower(m.Content), "!joke") {
		postDutchJokes(s, m)
		return
	}

	// check if this message was meant for our bot
	switch true {
	case strings.HasPrefix(strings.ToLower(m.Content), "!summary"), strings.HasPrefix(strings.ToLower(m.Content), "!drivers"):
	case strings.HasPrefix(strings.ToLower(m.Content), "!standings"), strings.HasPrefix(strings.ToLower(m.Content), "!rankings"):
	case strings.HasPrefix(strings.ToLower(m.Content), "!stats"), strings.HasPrefix(strings.ToLower(m.Content), "!statistics"):
	default:
		return
	}
	/*
	   !summary [series] [week]
	   !standings|ranking [series]
	   !stats [series] [week]
	*/

	var weekLookup, seriesLookup string
	// try to guess racing series by channel name
	c, err := s.Channel(m.ChannelID)
	if err != nil {
		log.Errorf("error getting channel: %v", err)
		return
	}
	switch true {
	case strings.Contains(strings.ToLower(c.Name), "adical"):
		seriesLookup = "Radical"
	case strings.Contains(strings.ToLower(c.Name), "indy"):
		seriesLookup = "Indy Pro"
	case strings.Contains(strings.ToLower(c.Name), "fr20"):
		seriesLookup = "Formula Renault 2.0"
	case strings.Contains(strings.ToLower(c.Name), "fr35"):
		seriesLookup = "Formula 3.5"
	case strings.Contains(strings.ToLower(c.Name), "f3"):
		seriesLookup = "F3 Championship"
	case strings.Contains(strings.ToLower(c.Name), "ir04"):
		seriesLookup = "Formula iR-04"
	case strings.Contains(strings.ToLower(c.Name), "1600"):
		seriesLookup = "Formula 1600"
	case strings.Contains(strings.ToLower(c.Name), "sf23"):
		seriesLookup = "Super Formula"
	}
	if len(seriesLookup) > 0 {
		log.Debugf("found series name by channel lookup: %s", seriesLookup)
	}

	var teamLookup string
	// try to guess team by server name
	g, err := s.Guild(c.GuildID)
	if err != nil {
		log.Errorf("error getting guild: %v", err)
		return
	}
	teamLookup = url.QueryEscape(g.Name)

	// eval parameters
	params := strings.Split(m.Content, " ")
	if len(seriesLookup) == 0 { // we haven't guessed series name yet
		if len(params) == 2 {
			// full specific series summary
			seriesLookup = params[1]
		}
		if len(params) == 3 {
			// specific week summary
			seriesLookup = params[1]
			weekLookup = params[2]
		}
	} else { // we've already guessed series by channel name
		if len(params) == 2 &&
			(!strings.HasPrefix(strings.ToLower(m.Content), "!standings") ||
				!strings.HasPrefix(strings.ToLower(m.Content), "!rankings")) {
			// specific week summary
			weekLookup = params[1]
		}
		if len(params) == 3 {
			// specific week summary
			seriesLookup = params[1]
			weekLookup = params[2]
		}
	}

	// verify parameters
	if len(weekLookup) > 0 {
		// strip all non-numeric characters
		weekLookup = rx.ReplaceAllString(weekLookup, "")
		// check validity
		if w, err := strconv.Atoi(weekLookup); err != nil || w < 1 || w > 13 {
			if _, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Invalid week number given: %v", weekLookup)); err != nil {
				log.Errorf("error sending message: %v", err)
				return
			}
			return
		}
	}

	// query current seasons
	series, err := getSeriesData()
	if err != nil {
		log.Errorf("error querying series data: %v", err)
		return
	}

	// process message
	if strings.HasPrefix(strings.ToLower(m.Content), "!summary") ||
		strings.HasPrefix(strings.ToLower(m.Content), "!drivers") {
		postSummary(s, m, teamLookup, weekLookup, seriesLookup, series)
	}
	if strings.HasPrefix(strings.ToLower(m.Content), "!standings") ||
		strings.HasPrefix(strings.ToLower(m.Content), "!rankings") {
		postStandings(s, m, teamLookup, weekLookup, seriesLookup, series)
	}
	if strings.HasPrefix(strings.ToLower(m.Content), "!stats") ||
		strings.HasPrefix(strings.ToLower(m.Content), "!statistics") {
		postStatistics(s, m, teamLookup, weekLookup, seriesLookup, series)
	}
}

func postDutchJokes(s *discordgo.Session, m *discordgo.MessageCreate) {
	// respond
	joke, err := getJoke()
	if err == nil && len(joke) > 0 {
		embed := discordgo.MessageEmbed{
			Title:       "Let's hear a random dutch joke",
			Description: joke,
		}
		if _, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed); err != nil {
			log.Errorf("error sending message: %v", err)
			return
		}
	} else {
		log.Errorf("error retrieving joke: %v", err)
		return
	}
}

func postSummary(s *discordgo.Session, m *discordgo.MessageCreate, teamLookup, weekLookup, seriesLookup string, series []Series) {
	// respond
	for _, season := range series {
		// full series summary
		if len(weekLookup) == 0 {
			if strings.Contains(strings.ToLower(season.Name), strings.ToLower(seriesLookup)) || len(seriesLookup) == 0 {
				embed := discordgo.MessageEmbed{
					Title:       fmt.Sprintf("%s - Driver Summary", season.CurrentSeason),
					Description: fmt.Sprintf("Shows driver summary data for the whole %s season", season.Name),
					Type:        discordgo.EmbedTypeImage,
					Image: &discordgo.MessageEmbedImage{
						URL: fmt.Sprintf(
							"https://irvisualizer.jamesclonk.io/season/%d/summary.png?team=%s&cb=%d",
							season.CurrentSeasonID, teamLookup, time.Now().UnixNano()/1000/1000/1000,
						),
					},
				}
				if _, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed); err != nil {
					log.Errorf("error sending message: %v", err)
					return
				}
			}
		} else { // specific week summary
			if strings.Contains(strings.ToLower(season.Name), strings.ToLower(seriesLookup)) || len(seriesLookup) == 0 {
				embed := discordgo.MessageEmbed{
					Title:       fmt.Sprintf("%s - Driver Summary - Week %s", season.CurrentSeason, weekLookup),
					Description: fmt.Sprintf("Shows driver summary data for week %s of the %s season", weekLookup, season.Name),
					Type:        discordgo.EmbedTypeImage,
					Image: &discordgo.MessageEmbedImage{
						URL: fmt.Sprintf(
							"https://irvisualizer.jamesclonk.io/season/%d/week/%s/summary.png?team=%s&cb=%d",
							season.CurrentSeasonID, weekLookup, teamLookup, time.Now().UnixNano()/1000/1000/1000,
						),
					},
				}
				if _, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed); err != nil {
					log.Errorf("error sending message: %v", err)
					return
				}
			}
		}
	}
}

func postStandings(s *discordgo.Session, m *discordgo.MessageCreate, teamLookup, weekLookup, seriesLookup string, series []Series) {
	// respond
	for _, season := range series {
		if strings.Contains(strings.ToLower(season.Name), strings.ToLower(seriesLookup)) || len(seriesLookup) == 0 {
			embed := discordgo.MessageEmbed{
				Title:       fmt.Sprintf("%s - Standings", season.Name),
				Description: fmt.Sprintf("Shows current standings for the %s", season.CurrentSeason),
				Type:        discordgo.EmbedTypeImage,
				Image: &discordgo.MessageEmbedImage{
					URL: fmt.Sprintf(
						"https://irvisualizer.jamesclonk.io/season/%d/standings.png?team=%s&cb=%d",
						season.CurrentSeasonID, teamLookup, time.Now().UnixNano()/1000/1000/1000,
					),
				},
			}
			if _, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed); err != nil {
				log.Errorf("error sending message: %v", err)
				return
			}
		}
	}
}

func postStatistics(s *discordgo.Session, m *discordgo.MessageCreate, teamLookup, weekLookup, seriesLookup string, series []Series) {
	// respond
	for _, season := range series {
		if strings.Contains(strings.ToLower(season.Name), strings.ToLower(seriesLookup)) || len(seriesLookup) == 0 {
			if len(weekLookup) == 0 {
				weekLookup = strconv.Itoa(season.CurrentWeek)
			}
			embed := discordgo.MessageEmbed{
				Title:       fmt.Sprintf("%s - Statistics - Week %s", season.CurrentSeason, weekLookup),
				Description: fmt.Sprintf("Shows statistics for week %s of the %s season", weekLookup, season.Name),
				Type:        discordgo.EmbedTypeImage,
				Image: &discordgo.MessageEmbedImage{
					URL: fmt.Sprintf(
						"https://irvisualizer.jamesclonk.io/season/%d/week/%s/top/scores.png?team=%s&cb=%d",
						season.CurrentSeasonID, weekLookup, teamLookup, time.Now().UnixNano()/1000/1000/1000,
					),
				},
			}
			if _, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed); err != nil {
				log.Errorf("error sending message: %v", err)
				return
			}
			embed = discordgo.MessageEmbed{
				Type: discordgo.EmbedTypeImage,
				Image: &discordgo.MessageEmbedImage{
					URL: fmt.Sprintf(
						"https://irvisualizer.jamesclonk.io/season/%d/week/%s/top/racers.png?team=%s&cb=%d",
						season.CurrentSeasonID, weekLookup, teamLookup, time.Now().UnixNano()/1000/1000/1000,
					),
				},
			}
			if _, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed); err != nil {
				log.Errorf("error sending message: %v", err)
				return
			}
			embed = discordgo.MessageEmbed{
				Type: discordgo.EmbedTypeImage,
				Image: &discordgo.MessageEmbedImage{
					URL: fmt.Sprintf(
						"https://irvisualizer.jamesclonk.io/season/%d/week/%s/top/safety.png?team=%s&cb=%d",
						season.CurrentSeasonID, weekLookup, teamLookup, time.Now().UnixNano()/1000/1000/1000,
					),
				},
			}
			if _, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed); err != nil {
				log.Errorf("error sending message: %v", err)
				return
			}
			embed = discordgo.MessageEmbed{
				Type: discordgo.EmbedTypeImage,
				Image: &discordgo.MessageEmbedImage{
					URL: fmt.Sprintf(
						"https://irvisualizer.jamesclonk.io/season/%d/week/%s/top/laps.png?team=%s&cb=%d",
						season.CurrentSeasonID, weekLookup, teamLookup, time.Now().UnixNano()/1000/1000/1000,
					),
				},
			}
			if _, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed); err != nil {
				log.Errorf("error sending message: %v", err)
				return
			}
		}
	}
}

type Series struct {
	ID              int    `json:"series_id"`
	Name            string `json:"name"`
	CurrentSeason   string `json:"current_season"`
	CurrentSeasonID int    `json:"current_season_id"`
	CurrentWeek     int    `json:"current_week"`
}

func getSeriesData() ([]Series, error) {
	resp, err := http.Get("https://irvisualizer.jamesclonk.io/series_json")
	if err != nil {
		return nil, fmt.Errorf("failed request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %v", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %v", err)
	}

	var series []Series
	if err := json.Unmarshal(data, &series); err != nil {
		log.Errorf("could not parse series json data: %#v", data)
		return nil, err
	}
	return series, nil
}

func getJoke() (string, error) {
	jokeType := "xxx"
	if rand.Intn(2) > 0 {
		jokeType = "nl"
	}

	resp, err := http.Get("http://api.apekool.nl/services/jokes/getjoke.php?type=" + jokeType)
	if err != nil {
		return "", fmt.Errorf("failed joke request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("joke status code: %v", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %v", err)
	}

	joke := struct {
		Joke string `json:"joke"`
	}{}
	if err := json.Unmarshal(data, &joke); err != nil {
		log.Errorf("could not parse joke json data: %#v", data)
		return "", err
	}
	return joke.Joke, nil
}
