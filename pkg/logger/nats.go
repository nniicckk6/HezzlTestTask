// Пакет logger предоставляет обёртку для публикации логов и событий в NATS
package logger

// Conn определяет минимальный интерфейс для работы с NATS-подключением
// Любая реализация Conn (например *nats.Conn) должна предоставлять метод Publish
// subject — тема (топик), data — байтовый массив сообщения
// Publish возвращает ошибку при неудаче публикации
type Conn interface {
	Publish(subject string, data []byte) error
}

// NATSClient хранит Conn и тему subject для публикации логов
type NATSClient struct {
	conn    Conn
	subject string
}

// NewClient создаёт новый NATSClient, связывая Conn и subject
func NewClient(conn Conn, subject string) *NATSClient {
	return &NATSClient{conn: conn, subject: subject}
}

// PublishLog отправляет данные в указанный subject в NATS
// Возвращает ошибку, если публикация не удалась
func (n *NATSClient) PublishLog(data []byte) error {
	return n.conn.Publish(n.subject, data)
}
