package types

type myErrors string

const (
	ErrNoAuth myErrors = "No authorization header"
)

func (e myErrors) Error() string {
	return string(e)
}
