package distance

const DefaultDistance = 2

func Hamming(str string, others []string) []string {
	var set []string
	for i := range others {
		if len(str) != len(others[i]) {
			continue
		}
		if dist := GetHammingDistance(str, others[i]); dist <= DefaultDistance {
			set = append(set, others[i])
		}
	}
	return set
}

func Levenshtein(str string, others []string) []string {
	var set []string
	for i := range others {
		if dist := GetLevenshteinDistance(str, others[i]); dist <= DefaultDistance {
			set = append(set, others[i])
		}
	}
	return set
}

func GetHammingDistance(fst, snd string) int {
	if fst == snd {
		return 0
	}
	var dist int
	for i := range fst {
		if fst[i] != snd[i] {
			dist++
		}
	}
	return dist
}

func GetLevenshteinDistance(fst, snd string) int {
	if fst == snd {
		return 0
	}
	if len(fst) == 0 {
		return len(snd)
	}
	if len(snd) == 0 {
		return len(fst)
	}
	var (
		zfst   = len(fst) + 1
		zsnd   = len(snd) + 1
		matrix = make([][]int, zsnd)
	)
	for i := range matrix {
		matrix[i] = make([]int, zfst)
	}
	for i := 0; i < zfst; i++ {
		matrix[0][i] = i
	}
	for i := 0; i < zsnd; i++ {
		matrix[i][0] = i
	}

	for i := 1; i < zfst; i++ {
		for j := 1; j < zsnd; j++ {
			var cost, del, ins, sub int
			if fst[i-1] != snd[j-1] {
				cost++
			}

			del = matrix[j-1][i] + 1
			ins = matrix[j][i-1] + 1
			sub = matrix[j-1][i-1] + cost
			matrix[j][i] = minimum(del, ins, sub)
		}
	}
	return matrix[zsnd-1][zfst-1]
}

func minimum(values ...int) int {
	var min int
	for i, v := range values {
		if i == 0 || v < min {
			min = v
		}
	}
	return min
}
