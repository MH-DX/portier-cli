package convert

import (
	"fmt"
	"strconv"
)

// nolint: goerr113
var errConversionError = func(v interface{}) error {
	err := fmt.Errorf("cannot convert value %v (type %T) to integer", v, v)
	return err
}

// ToInteger converts given value to integer.
func ToInteger(v interface{}) (int, error) {
	s := fmt.Sprintf("%v", v)
	i, err := strconv.Atoi(s)
	if err != nil {
		return i, errConversionError(v)
	}

	return i, nil
}
