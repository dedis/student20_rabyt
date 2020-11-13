package id

type Id interface {
	GetLength() byte
	GetBase() byte
	GetDigit(pos int) byte
}