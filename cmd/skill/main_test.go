package main

import (
	"bitbucket.org/sotavant/yandex-alice-skill/internal/store"
	"bitbucket.org/sotavant/yandex-alice-skill/internal/store/mock"
	"bytes"
	"compress/gzip"
	"github.com/go-resty/resty/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhook(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := mock.NewMockStore(ctrl)

	messages := []store.Message{
		{
			Sender:  "345345345345",
			Time:    time.Now(),
			Payload: "Hello",
		},
	}

	s.EXPECT().
		ListMessages(gomock.Any(), gomock.Any()).
		Return(messages, nil)

	appInstance := newApp(s)

	handler := http.HandlerFunc(appInstance.webhook)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	testCases := []struct {
		name         string
		method       string
		body         string
		expectedCode int
		expectedBody string
	}{
		{
			name:         "method_get",
			method:       http.MethodGet,
			expectedCode: http.StatusMethodNotAllowed,
			expectedBody: "",
		},
		{
			name:         "method_put",
			method:       http.MethodPut,
			expectedCode: http.StatusMethodNotAllowed,
			expectedBody: "",
		},
		{
			name:         "method_delete",
			method:       http.MethodDelete,
			expectedCode: http.StatusMethodNotAllowed,
			expectedBody: "",
		},
		{
			name:         "method_post_without_body",
			method:       http.MethodPost,
			expectedCode: http.StatusInternalServerError,
			expectedBody: "",
		},
		{
			name:         "method_post_unsupported_type",
			method:       http.MethodPost,
			body:         `{"request": {"type": "idunno", "command": "do something"}, "version": "1.0"}`,
			expectedCode: http.StatusUnprocessableEntity,
			expectedBody: "",
		},
		{
			name:         "method_post_success",
			method:       http.MethodPost,
			body:         `{"request": {"type": "SimpleUtterance", "command": "sudo do something"}, "session": {"new": true}, "version": "1.0"}`,
			expectedCode: http.StatusOK,
			expectedBody: `Точное время .* часов, .* минут. Для вас нет новых сообщений.`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := resty.New().R()
			r.Method = tc.method
			r.URL = srv.URL

			if len(tc.body) > 0 {
				r.SetHeader("Content-Type", "application/json")
				r.SetBody(tc.body)
			}

			resp, err := r.Send()
			assert.NoError(t, err, "error making request")

			assert.Equal(t, tc.expectedCode, resp.StatusCode(), "код ответа не совпадает")
			if tc.expectedBody != "" {
				assert.Regexp(t, tc.expectedBody, string(resp.Body()))
			}
		})
	}
}

func TestGzipCompression(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := mock.NewMockStore(ctrl)

	messages := []store.Message{
		{
			Sender:  "345345345345",
			Time:    time.Now(),
			Payload: "Hello",
		},
	}

	s.EXPECT().
		ListMessages(gomock.Any(), gomock.Any()).
		Return(messages, nil)

	appInstance := newApp(s)

	handler := gzipMiddleware(appInstance.webhook)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	requestBody := `{
		"request": {
			"type": "SimpleUtterance",
			"command": "sudo do something"
		},
		"version": "1.0"
	}`

	successBody := `{
		"response": {
			"text": "Для вас нет новых сообщений."
		},
		"version": "1.0"
	}`

	t.Run("sends_gzip", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		zb := gzip.NewWriter(buf)
		_, err := zb.Write([]byte(requestBody))
		require.NoError(t, err)
		err = zb.Close()
		require.NoError(t, err)

		r := httptest.NewRequest("POST", srv.URL, buf)
		r.RequestURI = ""
		r.Header.Set("Content-Encoding", "gzip")
		r.Header.Set("Accept-Encoding", "0")

		resp, err := http.DefaultClient.Do(r)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		defer func(Body io.ReadCloser) {
			err := Body.Close()
			require.NoError(t, err)
		}(resp.Body)

		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.JSONEq(t, successBody, string(b))
	})

	t.Run("accept_gzip", func(t *testing.T) {
		buf := bytes.NewBufferString(requestBody)
		r := httptest.NewRequest("POST", srv.URL, buf)
		r.RequestURI = ""
		r.Header.Set("Accept-Encoding", "gzip")

		resp, err := http.DefaultClient.Do(r)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		defer resp.Body.Close()

		zr, err := gzip.NewReader(resp.Body)
		require.NoError(t, err)

		b, err := io.ReadAll(zr)
		require.NoError(t, err)

		require.JSONEq(t, successBody, string(b))
	})
}
