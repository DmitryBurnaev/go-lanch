package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/ledongthuc/pdf"
)

const MainUrl = "https://puberty-spb.ru/menu/menyu-restorana/"

var savedMenus = map[string]string{}

func readPdf(path string) (string, error) {
	f, r, err := pdf.Open(path)
	// remember close file
	defer f.Close()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	n, err := buf.ReadFrom(b)
	if err != nil {
		return "", err
	}
	fmt.Printf("Read %d bytes from file '%s'\n", n, path)
	return buf.String(), nil
}

func downloadFile(url string, filepath string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func downloadMenu() string {
	// Request the HTML page.
	res, err := http.Get(MainUrl)
	if err != nil {
		log.Fatalf("Couldn't download %s: %s", MainUrl, err.Error())
		return ""
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatalf("Couldn't close HTML response body: %s", err.Error())
		}
	}(res.Body)

	if res.StatusCode != 200 {
		log.Fatalf("Status code error: %d %s", res.StatusCode, res.Status)
		return ""
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatalf("Couldn't parse HTML content: %s", err.Error())
	}

	menuURL := ""

	// Find the menu URL
	doc.Find("a.item-link").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if strings.Contains(href, "obed") || strings.Contains(href, "lanch") {
			fmt.Println(href)
			menuURL = href
		}
	})
	if menuURL == "" {
		log.Fatalf("Couldn't fetch menu URL from %s", MainUrl)
		return ""
	}
	tmpDir := os.TempDir()
	filePath := fmt.Sprintf("%s/%s", tmpDir, "paberti-obed.pdf")
	err = downloadFile(menuURL, filePath)
	if err != nil {
		return fmt.Sprintf("Couldn't download menu from URL: %s (%s)", menuURL, err)
	}
	return filePath
}

func getCurrentDayMenu() string {
	fmt.Println("getCurrentDayMenu")
	months := map[int]string{
		1: "января",
		4: "апреля",
		5: "мая",
		6: "июня",
		7: "июля",
	}
	separators := []string{
		"Салат дня – ",
		"Японский салат дня – ",
		"Суп дня – ",
		"Японский суп дня – ",
		"Горячее дня – ",
		"Напиток на выбор – ",
	}
	stopWord := "компот/пиво"

	currentTime := time.Now()
	nextTime := currentTime.AddDate(0, 0, 1)

	currentDay := fmt.Sprintf("%d %s", int(currentTime.Day()), months[int(currentTime.Month())])
	nextDay := fmt.Sprintf("%d %s", int(nextTime.Day()), months[int(nextTime.Month())])
	//
	//currentDay = "11 мая"
	//nextDay = "12 мая"

	currentMenu, exists := savedMenus[currentDay]
	if exists {
		log.Printf("Found saved menu for %s\n", currentDay)
		return currentMenu
	}

	log.Printf("Getting menu for %s...\n", currentDay)

	pdf.DebugOn = true
	menuPath := downloadMenu()
	if menuPath == "" {
		return "Couldn't download menu... sorry"
	}
	log.Printf("Parsing PDF %s...\n", menuPath)

	content, err := readPdf(menuPath) // Read local pdf file
	if err != nil {
		panic(err)
	}

	i0 := strings.Index(strings.ToLower(content), currentDay) + len(currentDay)
	i1 := strings.Index(strings.ToLower(content), nextDay)
	if i0 == -1 {
		currentMenu = "Меню на сегодня не найдено :("
		fmt.Println(content)
		log.Println("Couldn't find current day in downloaded menu. Skip sending.")
		return currentMenu
	} else {
		if i1 == -1 {
			currentMenu = content[i0:]
		} else {
			currentMenu = content[i0:i1]
		}
		fmt.Println(fmt.Sprintf("%s -> %s", currentDay, nextDay))
		fmt.Println(fmt.Sprintf("%d -> %d", i0, i1))
		for _, separator := range separators {
			newString := fmt.Sprintf("\n%s: ", strings.Replace(separator, " – ", "", 1))
			currentMenu = strings.ReplaceAll(currentMenu, separator, newString)
		}
		contentLastIndex := strings.Index(currentMenu, stopWord) + len(stopWord)
		currentMenu = currentMenu[:contentLastIndex]
	}
	currentMenu = fmt.Sprintf("Меню на %s\n\n%s", currentDay, currentMenu)
	fmt.Println(currentMenu)
	if err != nil {
		log.Fatalf("Couldn't send message to telegram chats: %s", err)
		return ""
	}
	savedMenus[currentDay] = currentMenu
	return currentMenu
}

func main() {
	log.Printf("Starting GoToPuberty BOT")

	bot, err := tgbotapi.NewBotAPI("1718325810:AAG3iF6X8OKLE9S7-3RTMnpamFLOotDRzbs")
	if err != nil {
		log.Panicf("Couldb't generate TG bot: %s", err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatalf("Couldn't get updates chan (telegram problems)")
		return
	}

	for update := range updates {
		println(update.Message.Text)
		if update.Message.Text != "/menu" {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		msg := getCurrentDayMenu()
		newMessage := tgbotapi.NewMessage(update.Message.Chat.ID, msg)
		//newMessage.ReplyToMessageID = update.Message.MessageID

		_, err := bot.Send(newMessage)
		if err != nil {
			log.Fatalf("Couldn't send message to channel %d. Error: %s", update.Message.Chat.ID, err)
		}
	}
}
