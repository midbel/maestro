package wrap

const MaxLenght = 72

func Wrap(str string) string {
  return WrapN(str, MaxLength)
}

func WrapN(str string, n int) string {
  return str
}
