package hexalog

import "fmt"

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
