package copyslice

func Copy[T any](arr []T) []T {
	ret := make([]T, len(arr))
	copy(ret, arr)
	return ret
}

func CopyMap[T comparable, V any](arr map[T]V) map[T]V {
	ret := make(map[T]V)
	for k, v := range arr {
		ret[k] = v
	}
	return ret
}
