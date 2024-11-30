package errhandler

// ErrorHandler handler for error
type ErrorHandler func(error)

// NotifyOrPanic if given errChan is nil, panic on error, otherwise send error
// to errChan
func NotifyOrPanic(errChan chan error) ErrorHandler {
	return func(err error) {
		if err == nil {
			return
		}
		if errChan != nil {
			errChan <- err
		} else {
			panic(err)
		}
	}
}
