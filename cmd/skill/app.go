package main

import (
	"bitbucket.org/sotavant/yandex-alice-skill/internal/logger"
	"bitbucket.org/sotavant/yandex-alice-skill/internal/models"
	"bitbucket.org/sotavant/yandex-alice-skill/internal/store"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

type app struct {
	store store.Store
}

func newApp(s store.Store) *app {
	return &app{store: s}
}

func (a *app) webhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		logger.Log.Debug("got request with bad method", zap.String("method", r.Method))

		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	logger.Log.Debug("decodint request")
	var req models.Request
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		logger.Log.Debug("cannto decode request JSON body", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if req.Request.Type != models.TypeSimpleUtterance {
		logger.Log.Debug("usupported request type", zap.String("type", req.Request.Type))
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	messages, err := a.store.ListMessages(ctx, req.Session.User.UserID)
	if err != nil {
		logger.Log.Debug("cannot load messages for user", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// формируем текст с количеством сообщений
	var text string
	switch true {
	case strings.HasPrefix(req.Request.Command, "Отправь"):
		username, message := parseSendCommand(req.Request.Command)

		recipientID, err := a.store.FindRecipient(ctx, username)
		if err != nil {
			logger.Log.Debug("cannot find recipient by username", zap.String("username", username), zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = a.store.SaveMessage(ctx, recipientID, store.Message{
			Sender:  req.Session.User.UserID,
			Time:    time.Now(),
			Payload: message,
		})

		if err != nil {
			logger.Log.Debug("cannot save message", zap.String("recipint", recipientID), zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		text = "Сообщение успешно отправлено"
	case strings.HasPrefix(req.Request.Command, "Прочитай"):
		messageIndex := parseReadCommand(req.Request.Command)

		messages, err := a.store.ListMessages(ctx, req.Session.User.UserID)
		if err != nil {
			logger.Log.Debug("cannot load messages for user", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		text = "Для вас нет новых сообщений."
		if len(messages) < messageIndex {
			// пользователь попросил прочитать сообщение, которого нет
			text = "Такого сообщения не существует."
		} else {
			// получим сообщение по идентификатору
			messageID := messages[messageIndex].ID
			message, err := a.store.GetMessage(ctx, messageID)
			if err != nil {
				logger.Log.Debug("cannot load message", zap.Int64("id", messageID), zap.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// передадим текст сообщения в ответе
			text = fmt.Sprintf("Сообщение от %s, отправлено %s: %s", message.Sender, message.Time, message.Payload)
		}
	case strings.HasPrefix(req.Request.Command, "Зарегистрируй"):
		username := parseRegisterCommand(req.Request.Command)
		err := a.store.RegisterUser(ctx, req.Session.User.UserID, username)
		if err != nil && !errors.Is(err, store.ErrConflict) {
			logger.Log.Debug("cannot register user", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		text = fmt.Sprintf("Вы успешно зарегистрированы под именем %s", username)
		if errors.Is(err, store.ErrConflict) {
			text = "Извините, такое имя уже занято. Попробуйте другое."
		}
	default:
		messages, err := a.store.ListMessages(ctx, req.Session.User.UserID)
		if err != nil {
			logger.Log.Debug("cannot load messages for user", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		text = "Для вас нет новых сообщений."
		if len(messages) > 0 {
			text = fmt.Sprintf("Для вас %d новых сообщений.", len(messages))
		}

		// первый запрос новой сессии
		if req.Session.New {
			// обработаем поле Timezone запроса
			tz, err := time.LoadLocation(req.Timezone)
			if err != nil {
				logger.Log.Debug("cannot parse timezone")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// получим текущее время в часовом поясе пользователя
			now := time.Now().In(tz)
			hour, minute, _ := now.Clock()

			// формируем новый текст приветствия
			text = fmt.Sprintf("Точное время %d часов, %d минут. %s", hour, minute, text)
		}
	}

	// заполним модель ответа
	resp := models.Response{
		Response: models.ResponsePayload{
			Text: text, // Алиса проговорит текст
		},
		Version: "1.0",
	}

	w.Header().Set("Content-Type", "application/json")

	// сериализуем ответ сервера
	enc := json.NewEncoder(w)
	if err := enc.Encode(resp); err != nil {
		logger.Log.Debug("error encoding response", zap.Error(err))
		return
	}
	logger.Log.Debug("sending HTTP 200 response")
}
