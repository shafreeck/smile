package unwrap

import "log"

func Err[T any](t T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return t
}

func Must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
