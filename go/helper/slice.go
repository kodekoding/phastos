package helper

func SliceContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func Remove(slice []interface{}, s int) []interface{} {
	return append(slice[:s], slice[s+1:]...)
}
