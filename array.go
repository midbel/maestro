package maestro

func copyStringArray(str [][]string, values []string) [][]string {
	if len(str) == 0 {
		for i := range values {
			str = append(str, []string{values[i]})
		}
		return str
	}
	var (
		old  = copyArray(str)
		list [][]string
	)
	for i := range values {
		arr := copyArray(old)
		for j := range arr {
			arr[j] = append(arr[j], values[i])
		}
		list = append(list, arr...)
	}
	return list
}

func copyArray(list [][]string) [][]string {
	var ret [][]string
	for i := range list {
		a := make([]string, len(list[i]))
		copy(a, list[i])
		ret = append(ret, a)
	}
	return ret
}
