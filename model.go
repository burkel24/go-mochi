package mochi

type Model interface {
	GetID() uint
	GetUserID() uint
	SetUserID(userID uint)
}