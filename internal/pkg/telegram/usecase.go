package telegram

//go:generate ./../../../bin/mockery --name Usecase
type Usecase interface {
	SendMessage(chatID string, message string) error
}
