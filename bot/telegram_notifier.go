package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	rmq "main/rabbitmq-client"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(FilterNumber(s), 64)
}

func ParseInt(s string) (int64, error) {
	if v, err := strconv.ParseInt(FilterNumber(s), 10, 64); err == nil {
		return v, nil
	}
	if v, err := ParseFloat(s); err == nil {
		if v >= 0.00 {
			return int64(math.Floor(v + 0.5)), nil
		}
		return int64(math.Ceil(v - 0.5)), nil
	}
	return int64(0), nil
}

var digitsOnly *regexp.Regexp

func FilterNumber(s string) string {
	if digitsOnly == nil {
		digitsOnly = regexp.MustCompile(`[0-9.-]+`)
	}
	return strings.Join(
		digitsOnly.FindAllString(
			strings.Replace(
				strings.Replace(s, ",", ".", -1),
				"\u2212", "-", 1,
			), -1,
		), "",
	)
}

// TBot Отправитель сообщений Telegram-бот (обёртка для https://github.com/go-telegram-bot-api/telegram-bot-api/v5).
type TBot struct {
	tgbot.BotAPI
	systemChats []string // Список адресов для системных уведомлений.
	adminChats  []string // Список адресов для админов.
	editorChats []string // Список адресов для уведомлений, предназначенных для редакторов.
	eventsChats []string // Список адресов для административных уведомлений.
	helpMessage string   // Строка с текстом справочного сообщения бота.

	mux         sync.Mutex
	TGChatState map[int64]StateChat

	EventsChan chan TEvent
}

type StateChat struct {
	AnswerMsgID int
	Cmd         string
}

// NewTBot Инициализация Telegram-бота из конфигурационных данных.
func NewTBot(cfg TBotConfig) *TBot {
	bot, botInitErr := tgbot.NewBotAPI(cfg.Token)
	if botInitErr != nil {
		logrus.Errorf(" tgbot.NewBotAPI(token: %s): %s", cfg.Token, botInitErr)
		return nil
	}
	if bot == nil {
		return nil
	}
	return &TBot{
		BotAPI:      *bot,
		systemChats: cfg.SystemChats[:],
		adminChats:  cfg.AdminChats[:],
		eventsChats: cfg.EventsChats[:],
		editorChats: cfg.EditorChats[:],
		helpMessage: fmt.Sprintf(`Вот что я могу:
ℹ️*Информация:*
- получить идентификаторы чата/автора: /me или /getme
`),
		TGChatState: map[int64]StateChat{},
		EventsChan:  make(chan TEvent, 10),
	}
}

func (bot *TBot) ChatState(msg *tgbot.Message) (StateChat, bool) {
	if msg == nil || msg.Chat.ID == 0 {
		return StateChat{}, false
	}
	bot.mux.Lock()
	state, exists := bot.TGChatState[msg.Chat.ID]
	if exists {
		bot.mux.Unlock()
		return state, exists
	}
	bot.mux.Unlock()
	return StateChat{}, false
}

func (bot *TBot) SetChatState(msg *tgbot.Message, state StateChat) {
	if msg == nil || msg.Chat.ID == 0 {
		return
	}
	bot.mux.Lock()
	bot.TGChatState[msg.Chat.ID] = state
	bot.mux.Unlock()
}

func (bot *TBot) ClearChatState(msg *tgbot.Message) {
	if msg == nil || msg.Chat.ID == 0 {
		return
	}
	bot.mux.Lock()
	if _, exists := bot.TGChatState[msg.Chat.ID]; exists {
		delete(bot.TGChatState, msg.Chat.ID)
	}
	bot.mux.Unlock()
}

// SendNotify Реализация интерфейса отправителя - отправить уведомление для соответствующего уровня (роли).
// subject - тема письма;
// body - содержимое письма;
// levels - уровни уведомлений (получатели определятся автоматически из указанных в конфигурации приложения).
func (bot *TBot) SendNotify(subject, body string, levels ...string) error {
	recipients := make([]string, 0)

	for _, lvl := range levels {
		switch lvl {

		}
	}

	recipients = append(recipients, bot.eventsChats...)

	if len(recipients) == 0 {
		return errors.New("не указаны получатели")
	}

	if subject != "" {
		body = fmt.Sprintf("#%s\n%s", strings.ReplaceAll(subject, " ", "_"), body)
	}

	for _, receipt := range recipients {
		chatID, _ := ParseInt(receipt)
		if chatID == 0 {
			continue
		}
		if _, err := bot.sendMessage(body, chatID, nil); err != nil {
			return err
		}
	}
	return nil
}

// SendMessage Реализация интерфейса отправителя - отправить сообщение конкретным получателям.
// subject - тема письма;
// body - содержимое письма;
// recipients - список получателей.
func (bot *TBot) SendMessage(subject, body string, recipients ...string) error {
	if len(recipients) == 0 {
		return errors.New("не указаны получатели")
	}

	if subject != "" {
		body = subject + "\n" + body
	}

	for _, receipt := range recipients {
		chatID, _ := ParseInt(receipt)
		if chatID == 0 {
			continue
		}
		if _, err := bot.sendMessage(body, chatID, nil); err != nil {
			return err
		}
	}
	return nil
}

func (bot *TBot) Call(recipient string, payload map[string]interface{}) (string, error) {
	return "", nil
}

// Отправить сообщение.
func (bot *TBot) sendMessage(text string, to int64, replyMarkup interface{}) (tgbot.Message, error) {
	if to == 0 || text == "" {
		return tgbot.Message{}, nil
	}
	logrus.Infof(`TBot.sendMessage(to: %d)`, to)

	msg := tgbot.NewMessage(to, text)
	msg.ParseMode = tgbot.ModeHTML
	msg.ReplyMarkup = replyMarkup

	return bot.BotAPI.Send(msg)
}

// Ответить на сообщение.
func (bot *TBot) replyMessage(text string, msg *tgbot.Message, replyMarkup interface{}) (tgbot.Message, error) {
	if msg == nil || text == "" {
		return tgbot.Message{}, nil
	}
	logrus.Infof(`TBot.replyMessage(msg: %d)`, msg.MessageID)

	replyMsg := tgbot.NewMessage(msg.Chat.ID, text)
	replyMsg.ReplyToMessageID = msg.MessageID
	replyMsg.ParseMode = tgbot.ModeHTML
	replyMsg.ReplyMarkup = replyMarkup

	return bot.BotAPI.Send(replyMsg)
}

// Отправить фото в сообщении.
func (bot *TBot) sendPhoto(caption string, photoBytes []byte, to int64) (tgbot.Message, error) {

	logrus.Infof(`TBot.sendPhoto(to: %d)`, to)
	caption = strings.TrimSpace(caption)

	photoFile := tgbot.FileBytes{
		Bytes: photoBytes,
	}
	msg := tgbot.NewPhoto(to, photoFile)
	msg.Caption = caption
	msg.ParseMode = tgbot.ModeHTML

	return bot.BotAPI.Send(msg)
}

// Ответить на сообщение фотографией.
func (bot *TBot) replyPhoto(text string, photoBytes []byte, msg *tgbot.Message, replyMarkup interface{}) (tgbot.Message, error) {
	if msg == nil || text == "" {
		return tgbot.Message{}, nil
	}
	logrus.Infof(`TBot.replyMessage(msg: %d)`, msg.MessageID)

	photoFile := tgbot.FileBytes{
		Bytes: photoBytes,
	}
	replyMsg := tgbot.NewPhoto(msg.Chat.ID, photoFile)
	replyMsg.Caption = text
	replyMsg.ReplyToMessageID = msg.MessageID
	replyMsg.ParseMode = tgbot.ModeHTML
	replyMsg.ReplyMarkup = replyMarkup

	return bot.BotAPI.Send(replyMsg)
}

// Отредактировать сообщение.
func (bot *TBot) editMessage(text string, msg *tgbot.Message, replyMarkup *tgbot.InlineKeyboardMarkup) (tgbot.Message, error) {
	if msg == nil || text == "" {
		return tgbot.Message{}, nil
	}
	logrus.Infof(`TBot.editMessage(msg: %d)`, msg.MessageID)

	editMsg := tgbot.NewEditMessageText(msg.Chat.ID, msg.MessageID, text)
	editMsg.ParseMode = tgbot.ModeHTML
	editMsg.ReplyMarkup = replyMarkup

	return bot.BotAPI.Send(editMsg)
}

// Отправить файл.
func (bot *TBot) sendFile(chat int64, caption, fileName string, file []byte, replyMarkup interface{}) (tgbot.Message, error) {
	if len(file) == 0 || chat == 0 {
		return tgbot.Message{}, nil
	}
	if strings.TrimSpace(fileName); fileName == "" {
		fileName = "attach"
	}
	logrus.Infof(`TBot.sendFile(fileName: "%s", to: %v)`, fileName, chat)
	msgDoc := tgbot.FileBytes{Name: fileName, Bytes: file}
	docConf := tgbot.NewDocument(chat, msgDoc)
	docConf.Caption = caption
	docConf.ParseMode = tgbot.ModeHTML
	docConf.ReplyMarkup = replyMarkup
	return bot.BotAPI.Send(docConf)
}

// Отправить группу файлов.
func (bot *TBot) sendMediaGroup(chat int64, text string, files []TAttach) (tgbot.Message, error) {
	if len(files) == 0 || chat == 0 {
		return tgbot.Message{}, nil
	}
	text = strings.TrimSpace(text)

	mgFiles := make([]interface{}, 0)
	for _, f := range files {
		if f.Type == "image" {
			photoFile := tgbot.FileBytes{
				Name:  f.Name,
				Bytes: f.Content,
			}
			mgFile := tgbot.NewPhoto(chat, photoFile)
			mgFiles = append(mgFiles, mgFile)
		} else {
			msgDoc := tgbot.FileBytes{
				Name:  f.Name,
				Bytes: f.Content,
			}
			mgFile := tgbot.NewDocument(chat, msgDoc)
			mgFiles = append(mgFiles, mgFile)
		}
	}

	mg := tgbot.NewMediaGroup(chat, mgFiles)

	mg.ChannelUsername = text

	return bot.BotAPI.Send(mg)
}

func (bot *TBot) SendEvent(evt TEvent) (tgbot.Message, error) {

	if evt.Recipient == 0 {
		return tgbot.Message{}, nil
	}

	if len(evt.Attaches) == 0 {
		return bot.sendMessage(evt.Text, evt.Recipient, nil)
	}

	if len(evt.Attaches) == 1 {
		oneAttach := evt.Attaches[0]
		if oneAttach.Type == "image" {
			return bot.sendPhoto(evt.Text, oneAttach.Content, evt.Recipient)
		}
		return bot.sendFile(evt.Recipient, evt.Text, oneAttach.Name, oneAttach.Content, nil)
	}

	return bot.sendMediaGroup(evt.Recipient, evt.Text, evt.Attaches)
}

// Реализация интерфейса io.Writer для получения логов.
func (bot *TBot) Write(p []byte) (n int, err error) {
	err = bot.SendNotify("Log", string(p))
	return 0, err
}

// RabbitHandler Плучение события из внешней очереди и трассировка его в свой канал событий.
func (bot *TBot) RabbitHandler(data []byte) error {
	var evt TEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return err
	}

	bot.EventsChan <- evt

	return nil
}

// Start Запустить Telegram-бота - инициализировать состояние, как синглтон в рамках приложения.
func (bot *TBot) Start(consumers ...interface{}) {

	for _, cnsm := range consumers {
		switch v := cnsm.(type) {
		case *rmq.RabbitClient:
			// Запустим "слушателя" Раббит, передав ему свой обработчик событий.
			go v.Consume("events", bot.RabbitHandler)
		}
	}

	go func() {
		for {
			val, ok := <-bot.EventsChan
			if ok == false {
				fmt.Println(val, ok, "<-- loop broke!")
				logrus.Errorf("%v, %v <-- loop broke!", val, ok)
				break
			} else {
				if _, err := bot.SendEvent(val); err != nil {
					logrus.Errorf(" bot.SendEvent(val: %v): %s", val, err)
				}
			}
		}
	}()

	updCfg := tgbot.NewUpdate(0)
	updCfg.Timeout = 60

	commands := []tgbot.BotCommand{
		{Command: "help", Description: "Справка"},
		{Command: "me", Description: "Информация о чате"},
	}
	commandsCfg := tgbot.NewSetMyCommands(commands...)
	bot.BotAPI.Request(commandsCfg)

	updates := bot.BotAPI.GetUpdatesChan(updCfg)
	for upd := range updates {
		if upd.CallbackQuery != nil {
			// Обработка коллбэк-кнопок.
		}
		if upd.Message == nil {
			// Нет сообщения - нечего искать.
			continue
		}
		if upd.Message.IsCommand() {
			args := make([]string, 0)
			for _, arg := range strings.Split(upd.Message.CommandArguments(), " ") {
				if arg = strings.TrimSpace(arg); arg == "" {
					continue
				}
				args = append(args, arg)
			}
			// Обработка команд, прилетевших боту.
			switch upd.Message.Command() {

			case "getme", "me":
				text := fmt.Sprintf(
					`Здравуствуйте!
Ваш telegramID: %d
Ваш chatID: %d`, upd.Message.From.ID, upd.Message.Chat.ID)
				if _, botErr := bot.replyMessage(text, upd.Message, nil); botErr != nil {
					logrus.Errorf("TBot.replyMessage(): %s", botErr)
				}
			case "help", "h", "?", "start":
				if _, botErr := bot.replyMessage(bot.helpMessage, upd.Message, nil); botErr != nil {
					logrus.Errorf("TBot.replyMessage(): %s", botErr)
				}

			default:
				if _, botErr := bot.replyMessage(bot.helpMessage, upd.Message, nil); botErr != nil {
					logrus.Errorf("TBot.replyMessage(): %s", botErr)
				}
			}
		}
	}
}

// TBotConfig Конфигурационные данные Telegram-бота.
type TBotConfig struct {
	Token       string
	SystemChats []string // Список адресов для системных уведомлений.
	AdminChats  []string // Список адресов для админов.
	EventsChats []string // Список адресов для административных уведомлений.
	EditorChats []string // Список адресов для уведомлений, предназначенных для редакторов.
}

type TAttach struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content []byte `json:"content"`
}

type TEvent struct {
	Text      string    `json:"text"`
	Attaches  []TAttach `json:"attaches"`
	Recipient int64     `json:"recipient"`
}
