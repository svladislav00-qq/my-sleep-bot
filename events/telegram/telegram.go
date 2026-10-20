package telegram

import (
	"errors"
	"sync"

	"github.com/svladilsav00-qq/my-sleep-bot/clients/telegram"
	"github.com/svladilsav00-qq/my-sleep-bot/events"
	"github.com/svladilsav00-qq/my-sleep-bot/lib/e"
	"github.com/svladilsav00-qq/my-sleep-bot/storage/postgres"
)

type Processor struct {
	tg      *telegram.Client
	offset  int
	storage postgres.Storage

	preparing     map[int64]Preparing
	preparingStat map[int64]PreparingStat
	mu            sync.Mutex
}

type Meta struct {
	ChatID int64
	UserID int64
}

var (
	ErrUnknownEventType = errors.New("unknown event type")
	ErrUnknownMetaType  = errors.New("unknown meta type")
)

func New(client *telegram.Client, storage postgres.Storage) *Processor {
	return &Processor{
		tg:        client,
		storage:   storage,
		preparing: make(map[int64]Preparing),
	}
}

func (p *Processor) Fetch(limit int) ([]events.Event, error) {
	updates, err := p.tg.Updates(p.offset, limit)
	if err != nil {
		return nil, e.Wrap("can't get events", err)
	}

	if len(updates) == 0 {
		return nil, nil
	}

	res := make([]events.Event, 0, len(updates))

	for _, u := range updates {
		res = append(res, event(u))
	}

	p.offset = updates[len(updates)-1].ID + 1

	return res, nil
}

func (p *Processor) Process(event events.Event) error {
	switch event.Type {
	case events.Message:
		return p.processMessage(event)
	default:
		return e.Wrap("can't process message", ErrUnknownEventType)
	}
}

func (p *Processor) processMessage(event events.Event) error {
	meta, err := meta(event)
	if err != nil {
		return e.Wrap("can't process message", err)
	}

}

func meta(event events.Event) (Meta, error) {
	res, ok := event.Meta.(Meta)
	if !ok {
		return Meta{}, e.Wrap("can't get meta", ErrUnknownMetaType)
	}

	return res, nil
}

func event(upd telegram.Update) events.Event {
	updType := fetchType(upd)

	res := events.Event{
		Type: fetchType(upd),
		Text: fetchText(upd),
	}

	if updType == events.Message {
		res.Meta = Meta{
			ChatID: int64(upd.Message.Chat.ID),
			UserID: int64(upd.Message.From.ID),
		}
	}

	return res
}

func fetchType(upd telegram.Update) events.Type {
	if upd.Message == nil {
		return events.Unknown
	}

	return events.Message
}

func fetchText(upd telegram.Update) string {
	if upd.Message == nil {
		return ""
	}

	return upd.Message.Text
}
