//go:build !windows

package instance

type Guard struct{}

func Acquire() (*Guard, error) { return &Guard{}, nil }
func (g *Guard) Close() error  { return nil }
