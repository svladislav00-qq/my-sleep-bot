package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/svladilsav00-qq/my-sleep-bot/lib/e"
	"github.com/svladilsav00-qq/my-sleep-bot/storage"
)

const (
	AddTimeCmd = "/addTime"
	HelpCmd    = "/help"
	StartCmd   = "/start"
	InfoCmd    = "/info"
	StatCmd    = "/stat"
)

type Preparing struct {
	userID    int64
	step      string
	sleepTime time.Time
	wakeTime  time.Time
}

type PreparingStat struct {
	userID   int64
	step     string
	duration int
}

const layout = "02.01.2006 15:04"

func (p *Processor) doCmd(text string, chatID int64, userID int64) error {
	text = strings.TrimSpace(text)

	log.Printf("got new command '%s' from user_id = '%d'", text, userID)

	// addTime: format "02.01.2006 15:04"
	// stat: /stat
	// help: /help
	// start: /start
	// info: /info

	switch text {
	case AddTimeCmd:
		return p.addTime(chatID, userID, text)
	case StatCmd:
		return p.stat()
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

	if strings.TrimSpace(text) == "/addTime" {
		p.mu.Lock()
		p.preparing[userID] = Preparing{
			userID: userID,
			step:   "sleep_time",
		}
		p.mu.Unlock()

		return p.tg.SendMessage(chatID, msgFellSleep)
	}

	p.mu.Lock()
	pr, ok := p.preparing[userID]
	p.mu.Unlock()

	if !ok {
		return p.tg.SendMessage(chatID, "Для начала выбери команду /addTime или введи её в чат")
	}

	switch pr.step {
	case "sleep_time":
		sleepTime, parseErr := parsSleepTime(text, time.Local)
		if parseErr != nil {
			return p.tg.SendMessage(chatID, msgWrongTime)
		}

		if sleepTime.After(time.Now()) {
			return p.tg.SendMessage(chatID, msgFutureTime)
		}

		pr.sleepTime = sleepTime
		pr.step = "wake_time"

		p.mu.Lock()
		p.preparing[userID] = pr
		p.mu.Unlock()

		return p.tg.SendMessage(chatID, msgWokeUp)

	case "wake_time":
		wakeTime, parseErr := parsSleepTime(text, time.Local)
		if parseErr != nil {
			return p.tg.SendMessage(chatID, msgWrongTime)
		}

		if !wakeTime.After(pr.sleepTime) {
			return p.tg.SendMessage(chatID, "Время пробуждения должно быть позже время засыпания. Попробуй ещё раз")
		}

		if wakeTime.After(time.Now()) {
			return p.tg.SendMessage(chatID, msgFutureTime)
		}

		pr.wakeTime = wakeTime
		pr.step = "confirm"

		p.mu.Lock()
		p.preparing[userID] = pr
		p.mu.Unlock()

		summary := fmt.Sprintf(
			"Проверь запись: %s - %s. Всё верно?",
			pr.sleepTime.Format("02.01.2006 15:04"),
			pr.wakeTime.Format("02.01.2006 15:04"),
		)

		return p.tg.SendMessage(chatID, summary)

	case "confirm":
		answer := strings.ToLower(strings.TrimSpace(text))
		if answer != "да" {
			p.mu.Lock()
			delete(p.preparing, userID)
			p.mu.Unlock()

			return p.tg.SendMessage(chatID, "Запись отменена")
		}

		record := storage.SleepRecord{
			UserID:       userID,
			FellAsleepAT: pr.sleepTime,
			WokeUpAt:     pr.wakeTime,
		}

		if err := p.storage.SaveRecord(context.Background(), &record); err != nil {
			return err
		}

		p.mu.Lock()
		delete(p.preparing, userID)
		p.mu.Unlock()

		duration := pr.wakeTime.Sub(pr.sleepTime)
		return p.tg.SendMessage(chatID, fmt.Sprintf(
			"Сохранил ✅ Продолжительность сна: %d ч %d мин.",
			int(duration.Hours()),
			int(duration.Minutes()),
		))

	default:
		p.mu.Lock()
		delete(p.preparing, userID)
		p.mu.Unlock()

		return p.tg.SendMessage(chatID, "Сценарий сбросился. Начни заново командой /addTime")
	}
}

func (p *Processor) stat(userID, chatID int64, text string) (err error) {
	defer func() { err = e.WrapIfErr("can't do command: view stat", err) }()

	if strings.TrimSpace(text) == "/stat" {
		p.mu.Lock()
		p.preparingStat[userID] = PreparingStat{
			userID: userID,
			step:   "chose_period",
		}
		p.mu.Unlock()

		return p.tg.SendMessage(chatID, msgStat)
	}

	p.mu.Lock()
	pr, ok := p.preparingStat[userID]
	p.mu.Unlock()

	if !ok {
		return p.tg.SendMessage(chatID, "Для начала выбери команду /stat или введи её в чат")
	}

	switch pr.step {
	case "chose_period":

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
