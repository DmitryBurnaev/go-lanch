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

	"errors"
)

const MainUrl = "https://puberty-spb.ru/menu/menyu-restorana/"

var separators = []string{
	"Салат дня – ",
	"Японский салат дня – ",
	"Суп дня – ",
	"Японский суп дня – ",
	"Горячее дня – ",
	"Напиток на выбор – ",
}
var months = map[int]string{
	1:  "января",
	4:  "апреля",
	5:  "мая",
	6:  "июня",
	7:  "июля",
	8:  "августа",
	9:  "сентября",
	10: "октября",
	11: "ноября",
	12: "декабря",
}
var mushroomWords = []string{
	"гриб",
	"рыбные зразы",
	"рыбные биточки",
	"польский соус",
	"с копченым куриным бедром и грудинкой",
	"полпетта",
}
var excludeWords = []string{
	"21 декабря",
	"22 декабря",
	"Акция 10 обед в подарок действует до 29.10.2021 года. Карточки на накопление обедов не выдаются с 17.09.2021 года.ВНИМАНИЕ!",
}

var savedMenus = map[string]string{}
var savedContents = map[string]string{}

// Allows to read text content from PDF file
func readPdf(filepath string) (string, error) {
	f, r, err := pdf.Open(filepath)
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatalf("Couldn't close read file: filepath: '%s' | error: '%s'", filepath, err.Error())
		}
	}(f)
	if err != nil {
		return "", err
	}

	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	n, err := buf.ReadFrom(b)
	if err != nil {
		return "", err
	}

	fmt.Printf("Read %d bytes from file '%s'", n, filepath)
	return buf.String(), nil
}

// Allows to download PDF from puberty's site
func downloadPDF(url string, filepath string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatalf("Couldn't close response body: '%s' | error: '%s'", url, err.Error())
		}
	}(resp.Body)

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			log.Fatalf("Couldn't close file handler: filepath: '%s' | error: '%s'", filepath, err.Error())
		}
	}(out)

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Printf("Couldn't write binary data to PDF file path: '%s' | error: '%s'", filepath, err.Error())
		return err
	}
	return nil
}

// Allows to download HTML with link to PDF
func downloadMenu() (string, error) {
	// Request the HTML page.
	res, err := http.Get(MainUrl)
	if err != nil {
		errMsg := fmt.Sprintf("Couldn't fetch url %s: '%s'", MainUrl, err.Error())
		log.Printf(errMsg)
		return "", errors.New(errMsg)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatalf("Couldn't close HTML response body: %s", err.Error())
		}
	}(res.Body)

	if res.StatusCode != 200 {
		errMsg := fmt.Sprintf(
			"Couldn't download menu: got unexpected status code for url: %s | status %d",
			MainUrl, res.StatusCode,
		)
		log.Printf(errMsg)
		return "", errors.New(errMsg)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		errMsg := fmt.Sprintf("Couldn't parse HTML content from url: %s | error %s", MainUrl, err.Error())
		log.Printf(errMsg)
		return "", errors.New(errMsg)
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
		errMsg := fmt.Sprintf("Couldn't find menu's url: %s", MainUrl)
		log.Printf(errMsg)
		return "", errors.New(errMsg)
	}
	tmpDir := os.TempDir()
	filePath := fmt.Sprintf("%s/%s", tmpDir, "paberti-obed.pdf")
	err = downloadPDF(menuURL, filePath)
	if err != nil {
		errMsg := fmt.Sprintf("Couldn't download menu from URL: %s | error '%s'", menuURL, err.Error())
		log.Printf(errMsg)
		return "", errors.New(errMsg)
	}
	return filePath, nil
}

// Allows to found requested day (today/tomorrow/...) in content, given from PDF
func fetchDay(content string, shiftDays int) string {
	currentMenu := ""
	currentDT := time.Now()
	currentDateTime := currentDT.AddDate(0, 0, shiftDays)
	nextDateTime := currentDT.AddDate(0, 0, shiftDays+1)
	currentDay := fmt.Sprintf("%d %s", currentDateTime.Day(), months[int(currentDateTime.Month())])
	nextDay := fmt.Sprintf("%d %s", nextDateTime.Day(), months[int(nextDateTime.Month())])

	log.Printf("Fetching menu for currentDay %s\n", currentDay)
	currentMenu, exists := savedMenus[currentDay]
	if exists {
		log.Printf("Found saved menu for %s\n", currentDay)
		return currentMenu
	}

	i0 := strings.Index(strings.ToLower(content), currentDay)
	i1 := strings.Index(strings.ToLower(content), nextDay)
	if i0 == -1 {
		log.Printf("Couldn't find day %s in downloaded menu. Skip day.\n", currentDay)
		return ""
	}

	i0 += len(currentDay)
	if i1 == -1 {
		currentMenu = content[i0:]
	} else {
		currentMenu = content[i0:i1]
	}
	for _, separator := range separators {
		newString := fmt.Sprintf("\n%s: ", strings.Replace(separator, " – ", "", 1))
		currentMenu = strings.ReplaceAll(currentMenu, separator, newString)
		currentMenu = strings.ReplaceAll(currentMenu, "�", "")
	}
	for _, word := range excludeWords {
		currentMenu = strings.ReplaceAll(currentMenu, word, "")
	}
	mushroomPostfix := ""
	for _, mushroomWord := range mushroomWords {
		mushroomIndexWords := strings.Index(strings.ToLower(currentMenu), mushroomWord)
		if mushroomIndexWords != -1 {
			mushroomPostfix = " 🍄 "
		}
	}
	currentMenu = fmt.Sprintf("Меню на %s%s\n\n%s\n\n===============\n", currentDay, mushroomPostfix, currentMenu)
	savedMenus[currentDay] = currentMenu
	return currentMenu
}

func getMenu(target string) string {
	var shiftDays []int
	switch target {
	case "today":
		shiftDays = []int{0}
	case "tomorrow":
		shiftDays = []int{1}
	case "week":
		shiftDays = []int{0, 1, 2, 3, 4, 5, 6}
	}

	currentDay := time.Now().Format("02-Jan-2006")
	content, contentCashExists := savedContents[currentDay]
	if !contentCashExists {
		log.Printf("Getting menu for %s...\n", target)
		menuPath, err := downloadMenu()
		if err != nil {
			return "Couldn't download menu... sorry"
		}

		log.Printf("Parsing PDF %s...\n", menuPath)
		content, err = readPdf(menuPath) // Read local pdf file
		if err != nil {
			log.Printf("Couldn't read PDF file: %s\n", err)
			return "Не получилось прочитать PDF с меню."
		}
	} else {
		log.Printf("Getting content from cashe. currentDay %s\n", currentDay)
	}

	resultMenu := ""
	for _, shiftDay := range shiftDays {
		currentMenu := fetchDay(content, shiftDay)
		if currentMenu != "" {
			resultMenu = fmt.Sprintf("\n%s%s", resultMenu, currentMenu)
			if !contentCashExists {
				savedContents[currentDay] = content
			}
		}
	}
	if resultMenu == "" {
		log.Printf("Couldn't find current day in downloaded menu. Skip sending. | Content: \n%s\n", content)
		return "Меню не найдено, ну или у них выходной :("
	}
	return resultMenu
}

func main() {
	log.Printf("Starting GoToPuberty BOT")
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TG_TOKEN"))
	if err != nil {
		log.Panicf("Couldb't start TG bot: %s", err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 80

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Printf("Couldn't get updates chan (telegram problems)")
		return
	}

	for update := range updates {
		commands := map[string]string{
			"/menu":     "today",
			"/tomorrow": "tomorrow",
			"/week":     "week",
		}
		msg := ""
		if command, ok := commands[update.Message.Text]; ok {
			log.Printf("[%s] %s -> %s", update.Message.From.UserName, update.Message.Text, command)
			msg = getMenu(command)
		} else {
			log.Printf("[%s] %s (unknown)", update.Message.From.UserName, update.Message.Text)
			msg = "Пока не знаю такой команды"
		}
		newMessage := tgbotapi.NewMessage(update.Message.Chat.ID, msg)
		_, err := bot.Send(newMessage)
		if err != nil {
			log.Printf("Couldn't send message to channel %d. Error: %s", update.Message.Chat.ID, err)
		}

	}
}
