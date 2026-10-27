package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/svladilsav00-qq/my-sleep-bot/clients/telegram"
	"github.com/svladilsav00-qq/my-sleep-bot/lib/e"
	"github.com/svladilsav00-qq/my-sleep-bot/storage"
)

const (
	addTimeCmd = "/addtime"
	HelpCmd    = "/help"
	StartCmd   = "/start"
	InfoCmd    = "/info"
	StatCmd    = "/stat"
)

type Preparing struct {
	userID int64
	step   string
	mode   string

	sleepTime time.Time
	wakeTime  time.Time

	proposedSleepTime time.Time
	proposedWakeTime  time.Time
}

type PreparingStat struct {
	userID int64
	step   string
}

const layout = "02.01.2006 15:04"

var userLocation = time.FixedZone("MSK", 3*60*60)

func (p *Processor) doCmd(text string, chatID int64, userID int64) error {
	text = strings.TrimSpace(text)

	log.Printf("got new command '%s' from user_id = '%d'", text, userID)

	p.mu.Lock()
	_, isPreparing := p.preparing[userID]
	_, isPreparingStat := p.preparingStat[userID]
	p.mu.Unlock()

	if isPreparing && text != addTimeCmd {
		return p.addTime(chatID, userID, text)
	}

	if isPreparingStat && text != StatCmd {
		return p.stat(userID, chatID, text)
	}
	switch text {
	case addTimeCmd:
		return p.addTime(chatID, userID, text)
	case StatCmd:
		return p.stat(userID, chatID, text)
	case InfoCmd:
		return p.sendInfo(chatID)
	case StartCmd:
		return p.sendHello(chatID)
	case HelpCmd:
		return p.sendHelp(chatID)
	default:
		return p.tg.SendMessage(chatID, msgUnknownCommand)
	}
}

func (p *Processor) addTime(chatID int64, userID int64, text string) (err error) {
	defer func() { err = e.WrapIfErr("can't do command: save time", err) }()

	if strings.TrimSpace(text) == "/addtime" {
		p.mu.Lock()
		p.preparing[userID] = Preparing{
			userID: userID,
			step:   "sleep_time",
			mode:   "create",
		}
		p.mu.Unlock()

		keyboard := telegram.InlineKeyboardMarkup{
			Keyboard: [][]telegram.InlineKeyboardButton{
				{
					{Text: "Отменить запись", CallbackData: "addtime:cancel"},
				},
			},
		}

		return p.tg.SendMessageWithButton(chatID, msgFellSleep, keyboard)
	}

	p.mu.Lock()
	pr, ok := p.preparing[userID]
	p.mu.Unlock()

	if !ok {
		return p.tg.SendMessage(chatID, msgAddErr)
	}

	switch pr.step {
	case "sleep_time":
		return p.handleSleepTime(chatID, userID, text, pr)
	case "wake_time":
		return p.handleWakeTime(chatID, userID, text, pr)
	case "confirm":
		return p.handleConfirm(chatID, userID, text, pr)
	case "edit":
		return p.handleEdit(chatID, userID, text, pr)
	case "choose_edit":
		return p.handleChoseEdit(chatID, userID, text, pr)
	default:
		log.Printf("addTime: unexpected mode=%q step=%q input=%q",
			pr.mode, pr.step, text)

		return p.tg.SendMessage(chatID, msgWrongTime)
	}
}

func (p *Processor) handleSleepTime(chatID, userID int64, text string, pr Preparing) error {
	if pr.mode == "edit" {
		if text == "addtime:cancelEdit" {
			pr.step = "confirm"

			p.setPreparing(userID, pr)

			return p.showConfirmation(chatID, pr.sleepTime, pr.wakeTime)
		}

		if text == "addtime:cancel" {
			p.mu.Lock()
			delete(p.preparing, userID)
			p.mu.Unlock()

			return p.tg.SendMessage(chatID, msgCancelRecord)
		}

		sleepTime, errorMessage := parseUserTime(text)
		if errorMessage != "" {
			return p.tg.SendMessage(chatID, errorMessage)
		}

		pr.sleepTime = sleepTime
		pr.step = "confirm"

		p.setPreparing(userID, pr)

		return p.showConfirmation(chatID, pr.sleepTime, pr.wakeTime)
	}

	if pr.mode == "editBoth" {
		if text == "addtime:cancelEdit" {
			pr.step = "confirm"

			p.setPreparing(userID, pr)

			return p.showConfirmation(chatID, pr.sleepTime, pr.wakeTime)
		}

		sleepTime, errorMessage := parseUserTime(text)
		if errorMessage != "" {
			return p.tg.SendMessage(chatID, errorMessage)
		}

		pr.proposedSleepTime = sleepTime
		pr.step = "wake_time"

		p.setPreparing(userID, pr)

		keyboard := editInputKeyboard()

		return p.tg.SendMessageWithButton(chatID, msgWokeUp, keyboard)
	}

	if text == "addtime:cancel" {
		p.mu.Lock()
		delete(p.preparing, userID)
		p.mu.Unlock()

		return p.tg.SendMessage(chatID, msgCancelRecord)
	}

	sleepTime, errorMessage := parseUserTime(text)
	if errorMessage != "" {
		return p.tg.SendMessage(chatID, errorMessage)
	}

	pr.sleepTime = sleepTime
	pr.step = "wake_time"

	p.setPreparing(userID, pr)

	keyboard := telegram.InlineKeyboardMarkup{
		Keyboard: [][]telegram.InlineKeyboardButton{
			{
				{Text: "Отменить запись", CallbackData: "addtime:cancel"},
			},
		},
	}

	return p.tg.SendMessageWithButton(chatID, msgWokeUp, keyboard)
}

func (p *Processor) handleWakeTime(chatID, userID int64, text string, pr Preparing) error {
	if pr.mode == "edit" {
		if text == "addtime:cancelEdit" {
			pr.step = "confirm"

			p.setPreparing(userID, pr)

			return p.showConfirmation(chatID, pr.sleepTime, pr.wakeTime)
		}

		if text == "addtime:cancel" {
			p.mu.Lock()
			delete(p.preparing, userID)
			p.mu.Unlock()

			return p.tg.SendMessage(chatID, msgCancelRecord)
		}

		wakeTime, parseErr := parsSleepTime(text, userLocation)
		if parseErr != nil {
			return p.tg.SendMessage(chatID, msgWrongTime)
		}

		if !wakeTime.After(pr.sleepTime) {
			return p.tg.SendMessage(chatID, msgFellWokeUpErr)
		}

		if wakeTime.After(time.Now().In(userLocation)) {
			return p.tg.SendMessage(chatID, msgFutureTime)
		}

		pr.wakeTime = wakeTime
		pr.step = "confirm"

		p.setPreparing(userID, pr)

		return p.showConfirmation(chatID, pr.sleepTime, pr.wakeTime)
	}

	if pr.mode == "editBoth" {
		if text == "addtime:cancelEdit" {
			pr.step = "confirm"

			p.setPreparing(userID, pr)

			return p.showConfirmation(chatID, pr.sleepTime, pr.wakeTime)
		}

		wakeTime, parseErr := parsSleepTime(text, userLocation)
		if parseErr != nil {
			return p.tg.SendMessage(chatID, msgWrongTime)
		}

		if !wakeTime.After(pr.proposedSleepTime) {
			return p.tg.SendMessage(chatID, msgFellWokeUpErr)
		}

		if wakeTime.After(time.Now().In(userLocation)) {
			return p.tg.SendMessage(chatID, msgFutureTime)
		}

		duration := wakeTime.Sub(pr.proposedSleepTime)
		if duration > 14*time.Hour {
			return p.tg.SendMessage(chatID, msgBadDuration)
		}

		pr.proposedWakeTime = wakeTime
		pr.step = "confirm"

		p.setPreparing(userID, pr)

		summary := fmt.Sprintf(
			"Проверь запись: %s - %s. Продолжительность сна : %d ч %d мин. Всё верно? да/нет",
			pr.proposedSleepTime.Format("02.01.2006 15:04"),
			pr.proposedWakeTime.Format("02.01.2006 15:04"),
			int(duration.Hours()),
			int(duration.Minutes())%60,
		)

		keyboard := telegram.InlineKeyboardMarkup{
			Keyboard: [][]telegram.InlineKeyboardButton{
				{
					{Text: "Да", CallbackData: "addtime:ok"},
					{Text: "Нет, изменить", CallbackData: "addtime:edit"},
					{Text: "Отменить изменения", CallbackData: "addtime:cancelEdit"},
				},
			},
		}

		return p.tg.SendMessageWithButton(chatID, summary, keyboard)
	}

	if text == "addtime:cancel" {
		p.mu.Lock()
		delete(p.preparing, userID)
		p.mu.Unlock()

		return p.tg.SendMessage(chatID, msgCancelRecord)
	}

	wakeTime, errorMessage := parseUserTime(text)
	if errorMessage != "" {
		return p.tg.SendMessage(chatID, errorMessage)
	}

	sleepTime := pr.sleepTime
	if pr.mode == "editBoth" {
		sleepTime = pr.proposedSleepTime
	}

	errorMessage = validateSleepInterval(sleepTime, wakeTime)
	if errorMessage != "" {
		return p.tg.SendMessage(chatID, errorMessage)
	}

	pr.wakeTime = wakeTime
	pr.step = "confirm"

	p.setPreparing(userID, pr)

	return p.showConfirmation(chatID, pr.sleepTime, pr.wakeTime)
}

func (p *Processor) handleConfirm(chatID, userID int64, text string, pr Preparing) error {
	if pr.mode == "editBoth" && text == "addtime:cancelEdit" {
		pr.proposedSleepTime = time.Time{}
		pr.proposedWakeTime = time.Time{}
		pr.step = "confirm"
		pr.mode = "create"

		p.setPreparing(userID, pr)

		return p.showConfirmation(chatID, pr.sleepTime, pr.wakeTime)
	}

	if text == "addtime:edit" {
		pr.step = "choose_edit"
		pr.mode = "edit"

		keyboard := telegram.InlineKeyboardMarkup{
			Keyboard: [][]telegram.InlineKeyboardButton{
				{
					{Text: "Изменить время когда уснул", CallbackData: "addtime:editFell"},
					{Text: "Изменить время когда проснулся", CallbackData: "addtime:editWoke"},
					{Text: "Изменить оба времени", CallbackData: "addtime:all"},
				},
			},
		}

		p.setPreparing(userID, pr)

		return p.tg.SendMessageWithButton(chatID, msgWhatEdit, keyboard)
	}

	if text == "addtime:no" {
		keyboard := telegram.InlineKeyboardMarkup{
			Keyboard: [][]telegram.InlineKeyboardButton{
				{
					{Text: "Исправить запись", CallbackData: "addtime:edit"},
					{Text: "Отменить запись", CallbackData: "addtime:cancel"},
				},
			},
		}

		pr.step = "edit"

		p.setPreparing(userID, pr)

		return p.tg.SendMessageWithButton(chatID, msgEditOrDelete, keyboard)
	}

	sleepTime := pr.sleepTime
	wakeTime := pr.wakeTime

	if pr.mode == "editBoth" {
		sleepTime = pr.proposedSleepTime
		wakeTime = pr.proposedWakeTime
	}

	if text != "addtime:ok" {
		return p.tg.SendMessage(chatID, "Выбери действие кнопкой выше")
	}

	record := storage.SleepRecord{
		UserID:       userID,
		FellAsleepAT: sleepTime,
		WokeUpAt:     wakeTime,
	}

	if err := p.storage.SaveRecord(context.Background(), &record); err != nil {
		return err
	}

	p.mu.Lock()
	delete(p.preparing, userID)
	p.mu.Unlock()

	duration := wakeTime.Sub(sleepTime)
	return p.tg.SendMessage(chatID, fmt.Sprintf(
		"Сохранил ✅\n\nПродолжительность сна: %d ч %d мин.",
		int(duration.Hours()),
		int(duration.Minutes())%60,
	))
}

func (p *Processor) handleEdit(chatID, userID int64, text string, pr Preparing) error {
	if text == "addtime:cancel" {
		p.mu.Lock()
		delete(p.preparing, userID)
		p.mu.Unlock()

		return p.tg.SendMessage(chatID, msgCancelRecord)
	}

	keyboard := telegram.InlineKeyboardMarkup{
		Keyboard: [][]telegram.InlineKeyboardButton{
			{
				{Text: "Изменить время когда уснул", CallbackData: "addtime:editFell"},
				{Text: "Изменить время когда проснулся", CallbackData: "addtime:editWoke"},
				{Text: "Изменить оба времени", CallbackData: "addtime:all"},
			},
		},
	}

	pr.step = "choose_edit"
	pr.mode = "edit"

	p.setPreparing(userID, pr)

	return p.tg.SendMessageWithButton(chatID, msgWhatEdit, keyboard)
}

func (p *Processor) handleChoseEdit(chatID, userID int64, text string, pr Preparing) error {
	switch text {
	case "addtime:editFell":
		pr.mode = "edit"
		pr.step = "sleep_time"

		keyboard := editInputKeyboard()

		p.setPreparing(userID, pr)

		return p.tg.SendMessageWithButton(chatID, msgFellSleep, keyboard)
	case "addtime:editWoke":
		pr.mode = "edit"
		pr.step = "wake_time"

		keyboard := editInputKeyboard()

		p.setPreparing(userID, pr)

		return p.tg.SendMessageWithButton(chatID, msgWokeUp, keyboard)
	case "addtime:all":
		pr.mode = "editBoth"
		pr.step = "sleep_time"
		pr.proposedSleepTime = time.Time{}
		pr.proposedWakeTime = time.Time{}

		keyboard := editInputKeyboard()

		p.setPreparing(userID, pr)

		return p.tg.SendMessageWithButton(chatID, msgFellSleep, keyboard)
	default:
		return p.tg.SendMessage(chatID, msgWrongTime)
	}
}

func (p *Processor) stat(userID, chatID int64, text string) (err error) {
	defer func() { err = e.WrapIfErr("can't do command: view stat", err) }()

	text = strings.TrimSpace(text)

	if strings.TrimSpace(text) == "/stat" {
		p.mu.Lock()
		p.preparingStat[userID] = PreparingStat{
			userID: userID,
			step:   "chose_period",
		}
		p.mu.Unlock()

		keyboard := telegram.InlineKeyboardMarkup{
			Keyboard: [][]telegram.InlineKeyboardButton{
				{
					{Text: "7 дней", CallbackData: "stat:7"},
					{Text: "14 дней", CallbackData: "stat:14"},
					{Text: "Месяц", CallbackData: "stat:month"},
				},
			},
		}

		return p.tg.SendMessageWithButton(chatID, msgStat, keyboard) // sendButtonStat
	}

	p.mu.Lock()
	pr, ok := p.preparingStat[userID]
	p.mu.Unlock()

	if !ok {
		return p.tg.SendMessage(chatID, "Для начала выбери команду /stat или введи её в чат")
	}

	switch pr.step {
	case "chose_period":
		from, to, ok := statPeriod(text, time.Now().In(userLocation))
		if !ok {
			return p.tg.SendMessage(chatID, msgStatErr)
		}

		records, err := p.storage.GetRecords(context.Background(), userID, from, to)
		if err != nil {
			return err
		}

		p.mu.Lock()
		delete(p.preparingStat, userID)

		if len(records) == 0 {
			return p.tg.SendMessage(chatID, msgNoRecords)
		}

		message := formatStat(records, from, to)
		return p.tg.SendMessage(chatID, message)
	default:
		p.mu.Lock()
		delete(p.preparingStat, userID)
		p.mu.Unlock()

		return p.tg.SendMessage(chatID, msgStatErr)
	}
}

func (p *Processor) sendInfo(chatID int64) error {
	return p.tg.SendMessage(chatID, msgInfo)
}

func (p *Processor) sendHello(chatID int64) error {
	return p.tg.SendMessage(chatID, msgHello)
}

func (p *Processor) sendHelp(chatID int64) error {
	return p.tg.SendMessage(chatID, msgHelp)
}

func parsSleepTime(text string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation(layout, strings.TrimSpace(text), loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("Ожидаю дату в формате ДД.ММ.ГГГГ ЧЧ:ММ: %w", err)
	}

	return t, nil
}

func (p *Processor) showConfirmation(chatID int64, sleepTime, wakeTime time.Time) error {
	duration := wakeTime.Sub(sleepTime)
	maxSleepDuration := 14 * time.Hour

	if duration > maxSleepDuration {
		return p.tg.SendMessage(chatID, msgBadDuration)
	}

	keyboard := telegram.InlineKeyboardMarkup{
		Keyboard: [][]telegram.InlineKeyboardButton{
			{
				{Text: "Да", CallbackData: "addtime:ok"},
				{Text: "Нет", CallbackData: "addtime:no"},
			},
		},
	}

	summary := fmt.Sprintf(
		"Проверь запись: %s - %s.\nПродолжительность сна : %d ч %d мин. Всё верно?",
		sleepTime.Format("02.01.2006 15:04"),
		wakeTime.Format("02.01.2006 15:04"),
		int(duration.Hours()),
		int(duration.Minutes())%60,
	)
	return p.tg.SendMessageWithButton(chatID, summary, keyboard)
}

func statPeriod(text string, now time.Time) (time.Time, time.Time, bool) {
	var duration time.Duration

	switch strings.TrimSpace(text) {
	case "stat:7":
		duration = 7 * 24 * time.Hour
	case "stat:14":
		duration = 14 * 24 * time.Hour
	case "stat:month":
		duration = 30 * 24 * time.Hour
	default:
		return time.Time{}, time.Time{}, false
	}

	return now.Add(-duration), now, true
}

func formatStat(records []storage.SleepRecord, from, to time.Time) string {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Сводка за %s - %s 💤\nНайдено записей: %d\n\n",
		from.Format("02.01.2006"),
		to.Format("02.01.2006"),
		len(records),
	)

	for _, record := range records {
		duration := record.WokeUpAt.Sub(record.FellAsleepAT)
		hours := int(duration / time.Hour)
		minutes := int((duration % time.Hour) / time.Minute)

		fmt.Fprintf(
			&b,
			"%s — %s · %d ч %d мин\n",
			record.FellAsleepAT.In(userLocation).Format("02.01.2006 15:04"),
			record.WokeUpAt.In(userLocation).Format("02.01.2006 15:04"),
			hours,
			minutes,
		)
	}

	return strings.TrimSpace(b.String())
}

func parseUserTime(text string) (time.Time, string) {
	t, err := parsSleepTime(text, userLocation)
	if err != nil {
		return time.Time{}, msgWrongTime
	}

	if t.After(time.Now().In(userLocation)) {
		return time.Time{}, msgFutureTime
	}

	return t, ""
}

func validateSleepInterval(sleepTime, wakeTime time.Time) string {
	if !wakeTime.After(sleepTime) {
		return msgFellWokeUpErr
	}

	duration := wakeTime.Sub(sleepTime)
	if duration > 14*time.Hour {
		return msgBadDuration
	}

	return ""
}

func (p *Processor) setPreparing(userID int64, pr Preparing) {
	p.mu.Lock()
	p.preparing[userID] = pr
	p.mu.Unlock()
}

func editInputKeyboard() telegram.InlineKeyboardMarkup {
	return telegram.InlineKeyboardMarkup{
		Keyboard: [][]telegram.InlineKeyboardButton{
			{
				{Text: "Отменить изменение", CallbackData: "addtime:cancelEdit"},
				{Text: "Отменить запись", CallbackData: "addtime:cancel"},
			},
		},
	}
}
