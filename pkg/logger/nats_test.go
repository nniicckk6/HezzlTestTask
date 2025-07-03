// Пакет logger содержит unit-тесты для проверки работы NATSClient и метода PublishLog
package logger

import (
	"bytes"
	"errors"
	"testing"
)

// mockConn реализует интерфейс Conn и позволяет перехватывать вызовы Publish
// Мы сохраняем переданный subject и данные для проверки в тестах
type mockConn struct {
	publishedSubject string // тема, переданная в Publish
	publishedData    []byte // данные, переданные в Publish
	returnErr        error  // ошибка, которую вернет Publish
}

// Publish сохраняет параметры вызова в полях mockConn и возвращает заранее заданную ошибку
func (m *mockConn) Publish(subject string, data []byte) error {
	m.publishedSubject = subject
	m.publishedData = data
	return m.returnErr
}

// TestPublishLog_Success проверяет успешную публикацию данных
// Проверяем, что PublishLog корректно вызывает Publish с тем же subject и данными без ошибок
func TestPublishLog_Success(t *testing.T) {
	subject := "test.subject"
	data := []byte("payload")
	mock := &mockConn{returnErr: nil}
	client := NewClient(mock, subject)

	err := client.PublishLog(data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mock.publishedSubject != subject {
		t.Errorf("expected subject %s, got %s", subject, mock.publishedSubject)
	}
	if string(mock.publishedData) != string(data) {
		t.Errorf("expected data %s, got %s", data, mock.publishedData)
	}
}

// TestPublishLog_Error проверяет прокидку ошибки из Conn.Publish
// Если underlying Publish возвращает ошибку, PublishLog должен вернуть ту же ошибку
func TestPublishLog_Error(t *testing.T) {
	subject := "test.subject"
	data := []byte("payload")
	expErr := errors.New("publish failed")
	mock := &mockConn{returnErr: expErr}
	client := NewClient(mock, subject)

	err := client.PublishLog(data)
	if !errors.Is(err, expErr) {
		t.Errorf("expected error %v, got %v", expErr, err)
	}
}

// TestPublishLog_EmptySubject проверяет сценарий с пустым subject
// Subject может быть пустой строкой, в этом случае PublishLog просто передаст пустой topic
func TestPublishLog_EmptySubject(t *testing.T) {
	data := []byte("data")
	mock := &mockConn{returnErr: nil}
	client := NewClient(mock, "")

	err := client.PublishLog(data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mock.publishedSubject != "" {
		t.Errorf("expected empty subject, got %s", mock.publishedSubject)
	}
	if !bytes.Equal(mock.publishedData, data) {
		t.Errorf("expected data %v, got %v", data, mock.publishedData)
	}
}

// TestPublishLog_NilData проверяет передачу nil в качестве данных
// PublishLog должен корректно передать nil, без паники и ошибок
func TestPublishLog_NilData(t *testing.T) {
	subject := "subj"
	mock := &mockConn{returnErr: nil}
	client := NewClient(mock, subject)

	err := client.PublishLog(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mock.publishedSubject != subject {
		t.Errorf("expected subject %s, got %s", subject, mock.publishedSubject)
	}
	if mock.publishedData != nil {
		t.Errorf("expected nil data, got %v", mock.publishedData)
	}
}
