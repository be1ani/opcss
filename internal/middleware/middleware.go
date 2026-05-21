// Middleware provides composable net/http middleware for the opcss.
package middleware

import "net/http"

func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	// reverse order: execute left to right
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
