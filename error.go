package handler

type RetryableError interface {
	IsRetryable() bool
}

func IsErrorRetryable(err error) bool {
	for err != nil {
		if rerr, ok := err.(RetryableError); ok {
			return rerr.IsRetryable()
		}
		// Unwrap the error if possible
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = unwrapper.Unwrap()
	}
	return false
}
