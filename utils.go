package hexalog

import "fmt"

func mergeErrors(e1, e2 error) (err error) {
	if e1 != nil && e2 != nil {
		return fmt.Errorf("%v; %v", e1, e2)
	} else if e1 != nil {
		return e1
	}
	return e2
}
