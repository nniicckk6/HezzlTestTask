package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"HezzlTestTask/internal/model"
)

// mockRepo реализует интерфейс Repo и сохраняет полученные события для проверки
type mockRepo struct {
	received [][]model.Good // полученные батчи событий
	err      error          // ошибка, которую вернет BatchInsertLogs
}

func (m *mockRepo) BatchInsertLogs(ctx context.Context, events []model.Good) error {
	// сохраняем копию слайса для проверки
	copyBatch := make([]model.Good, len(events))
	copy(copyBatch, events)
	m.received = append(m.received, copyBatch)
	return m.err
}

func TestHandleMessage_NoFlush(t *testing.T) {
	// тестируем, что при количестве событий меньше batchSize нет записи в репозиторий
	repo := &mockRepo{}
	cons := NewConsumer(repo, 3)

	// готовим событие
	data, _ := json.Marshal(model.Good{ID: 1, ProjectID: 10, Name: "g1"})
	err := cons.HandleMessage(context.Background(), data)
	require.NoError(t, err)
	// репозиторий не должен был быть вызван
	require.Len(t, repo.received, 0)
}

func TestHandleMessage_FlushOnBatch(t *testing.T) {
	// тестируем, что при достижении batchSize события отправляются репозиторию
	repo := &mockRepo{}
	cons := NewConsumer(repo, 2)

	// два события подряд приводят к одной записи
	for i := 1; i <= 2; i++ {
		e := model.Good{ID: i, ProjectID: 5, Name: "name"}
		data, _ := json.Marshal(e)
		err := cons.HandleMessage(context.Background(), data)
		require.NoError(t, err)
	}
	// проверяем, что репозиторий был вызван один раз
	require.Len(t, repo.received, 1)
	// проверяем содержимое батча
	require.Len(t, repo.received[0], 2)
	require.Equal(t, repo.received[0][0].ID, 1)
	require.Equal(t, repo.received[0][1].ID, 2)
}

func TestFlush_Empty(t *testing.T) {
	// тестируем, что Flush ничего не делает, если буфер пуст
	repo := &mockRepo{}
	cons := NewConsumer(repo, 5)
	err := cons.Flush(context.Background())
	require.NoError(t, err)
	require.Len(t, repo.received, 0)
}

func TestFlush_NonEmpty(t *testing.T) {
	// тестируем, что Flush отправляет накопленные события
	repo := &mockRepo{}
	cons := NewConsumer(repo, 5)

	// добавляем три события вручную через HandleMessage
	for i := 1; i <= 3; i++ {
		e := model.Good{ID: i, ProjectID: 2, Name: "n"}
		data, _ := json.Marshal(e)
		err := cons.HandleMessage(context.Background(), data)
		require.NoError(t, err)
	}
	// репозиторий ещё не должен быть вызван, т.к. batchSize=5
	require.Len(t, repo.received, 0)

	// вызов Flush
	err := cons.Flush(context.Background())
	require.NoError(t, err)
	require.Len(t, repo.received, 1)
	require.Len(t, repo.received[0], 3)
}

func TestHandleMessage_ParseError(t *testing.T) {
	// тестируем ошибку парсинга некорректного JSON
	repo := &mockRepo{}
	cons := NewConsumer(repo, 1)
	err := cons.HandleMessage(context.Background(), []byte("not json"))
	require.Error(t, err)
	// репозиторий не вызывался
	require.Len(t, repo.received, 0)
}

func TestBatchInsertError_IsPropagated(t *testing.T) {
	// тестируем, что ошибка из репозитория возвращается при достижении batchSize
	ex := errors.New("insert failed")
	repo := &mockRepo{err: ex}
	cons := NewConsumer(repo, 1)
	// batchSize=1, одно сообщение сразу вызывает BatchInsertLogs
	data, _ := json.Marshal(model.Good{ID: 9, ProjectID: 3, Name: "x"})
	err := cons.HandleMessage(context.Background(), data)
	require.Error(t, err)
	require.ErrorIs(t, err, ex)
}
