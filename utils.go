package hexalog

import "fmt"

// assumes equal length
func equalBytes(a, b []byte) bool {

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func mergeErrors(e1, e2 error) (err error) {

	if e1 == nil && e2 == nil {
	} else if e1 == nil {
		err = e2
	} else if e2 == nil {
		err = e1
	} else {
		err = fmt.Errorf("%v; %v", e1, e2)
	}

	return err
}
