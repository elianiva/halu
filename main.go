package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	tb "gopkg.in/tucnak/telebot.v2"
)

type AIResponse struct {
	Choices []struct {
		Text string
	}
}

type RequestBody struct {
	Prompt      string   `json:"prompt"`
	MaxTokens   uint     `json:"max_tokens"`
	Temperature uint     `json:"temperature"`
	Stop        []string `json:"stop,omitempty"`
	Engine      string   `json:"-"`
}

var ctxStore = map[int]string{}

func hasName(name string) bool {
	return name == "yuki" || name == "nishiyama"
}

func main() {
	b, err := tb.NewBot(tb.Settings{
		Token:  os.Getenv("BOT_KEY"),
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		panic(err)
	}

	b.Handle(tb.OnText, func(m *tb.Message) {
		msg := strings.Split(m.Text, " ")
		name := strings.ToLower(msg[0])

		var prompt, manCtx, yukiCtx string
		if hasName(name) && !m.IsReply() {
			prompt = strings.Join(msg[1:], " ")
			manCtx = ctxStore[m.Sender.ID]
		} else if m.IsReply() && m.ReplyTo.Sender.ID == b.Me.ID {
			prompt = strings.Join(msg, " ")
			manCtx = ctxStore[m.Sender.ID]

			if m.IsReply() {
				fmt.Println("Prev: ", yukiCtx, manCtx, ctxStore)

				manCtx = ctxStore[m.Sender.ID]
				yukiCtx = m.ReplyTo.Text

				// save for future context
				ctxStore[m.Sender.ID] = prompt

				fmt.Println("Current: ", yukiCtx, manCtx, ctxStore)
			}
		} else if m.Private() {
			prompt = strings.Join(msg, " ")
		} else {
			return
		}

		reply, err := getReply(name, prompt, manCtx, yukiCtx)
		if err != nil {
			b.Reply(m, "Hehehe.. aku lagi linglung")
			log.Printf("%v", err)
		}

		re := regexp.MustCompile("(?i)(Yuki):")
		b.Reply(m, re.ReplaceAllString(reply, ""))
	})

	go func() {
		b.Start()
	}()
	fmt.Println("Launching bot...")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("Terminating...")
}

func getReply(kind, prompt, manCtx, yukiCtx string) (string, error) {
	bot := getYuki(prompt, manCtx, yukiCtx)

	b, _ := json.Marshal(bot)

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/engines/"+bot.Engine+"/completions", bytes.NewBuffer(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_KEY"))
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	var data AIResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", err
	}

	return string(data.Choices[0].Text), nil
}

func getYuki(prompt, manCtx, yukiCtx string) RequestBody {
	req := RequestBody{
		Engine: "davinci",
		Prompt: "Below is a conversation between lovers. The man is charming while the woman is very friendly, lovely, and sweet. Here name is Yuki" +
			"\n" +
			"Man: Will you be my princess?\n" +
			"Yuki: Your princess? :3\n" +
			"Man: Yes :)\n" +
			"Yuki: What do the princess get?\n" +
			"Man: Love, happiness, and a happy ending.\n" +
			"Yuki: A happy ending? Do they exists?\n" +
			"Man: Yes, if you have me on your side :)\n" +
			"Yuki: Aww :3\n",
		MaxTokens:   64,
		Temperature: 1,
		Stop:        []string{"\n", "\nYuki:", "\nMan:"},
	}

	if manCtx != "" && yukiCtx != "" {
		req.Prompt += "Man: " + manCtx + "\n" +
			"Yuki: " + yukiCtx + "\n" +
			"Man: " + prompt + "\n"
	} else {
		req.Prompt += "Man: " + prompt + "\n"
	}

	return req
}
