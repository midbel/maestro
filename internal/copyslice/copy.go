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

func CopyValues[T any](arr [][]T, values []T) [][]T {
	if len(arr) == 0 {
		for i := range values {
			arr = append(arr, []T{values[i]})
		}
		return arr
	}
	var (
		old  = copyValues(arr)
		list [][]T
	)
	for i := range values {
		arr := copyValues(old)
		for j := range arr {
			arr[j] = append(arr[j], values[i])
		}
		list = append(list, arr...)
	}
	return list
}

func copyValues[T any](list [][]T) [][]T {
	var ret [][]T
	for i := range list {
		a := make([]T, len(list[i]))
		copy(a, list[i])
		ret = append(ret, a)
	}
	return ret
}
