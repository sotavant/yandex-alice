package main

import (
	"bitbucket.org/sotavant/yandex-alice-skill/internal/logger"
	"bitbucket.org/sotavant/yandex-alice-skill/internal/models"
	"bitbucket.org/sotavant/yandex-alice-skill/internal/store"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/http"
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
	text := "Для вас нет новых сообщений."
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

	// заполним модель ответа
	resp := models.Response{
		Response: models.ResponsePayload{
			Text: text, // Алиса проговорит наш новый текст
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
