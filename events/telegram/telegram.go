package telegram

import (
	"errors"
	"sync"

	"github.com/svladilsav00-qq/my-sleep-bot/clients/telegram"
	"github.com/svladilsav00-qq/my-sleep-bot/events"
	"github.com/svladilsav00-qq/my-sleep-bot/lib/e"
	"github.com/svladilsav00-qq/my-sleep-bot/storage"
)

type Processor struct {
	tg      *telegram.Client
	offset  int
	storage storage.Storage

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

func New(client *telegram.Client, storage storage.Storage) *Processor {
	return &Processor{
		tg:            client,
		storage:       storage,
		preparing:     make(map[int64]Preparing),
		preparingStat: make(map[int64]PreparingStat),
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
	case events.Callback:
		return p.processCallback(event)
	default:
		return e.Wrap("can't process message", ErrUnknownEventType)
	}
}

func (p *Processor) processMessage(event events.Event) error {
	meta, err := meta(event)
	if err != nil {
		return e.Wrap("can't process message", err)
	}

	if err := p.doCmd(event.Text, meta.ChatID, meta.UserID); err != nil {
		return e.Wrap("can't process message", err)
	}

	return nil
}

func meta(event events.Event) (Meta, error) {
	res, ok := event.Meta.(Meta)
	if !ok {
		return Meta{}, e.Wrap("can't get meta", ErrUnknownMetaType)
	}

	return res, nil
}

func event(upd telegram.Update) events.Event {
	if callback := upd.CallBackQuery; callback != nil {
		if callback.Message == nil {
			return events.Event{Type: events.Unknown}
		}

		return events.Event{
			Type: events.Callback,
			Text: callback.Data,
			Meta: Meta{
				ChatID: int64(callback.Message.Chat.ID),
				UserID: int64(callback.From.ID),
			},
		}
	}

	if message := upd.Message; message != nil {
		return events.Event{
			Type: events.Message,
			Text: message.Text,
			Meta: Meta{
				ChatID: int64(message.Chat.ID),
				UserID: int64(message.From.ID),
			},
		}
	}

	return events.Event{Type: events.Unknown}
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

func (p *Processor) processCallback(event events.Event) error {
	meta, err := meta(event)
	if err != nil {
		return e.Wrap("can't process callback", err)
	}

	return p.doCmd(event.Text, meta.ChatID, meta.UserID)
}
