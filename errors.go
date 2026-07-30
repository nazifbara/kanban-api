package main

type multiErr interface {
	error
	Unwrap() []error
}
