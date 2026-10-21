package main

import "fmt"

func requirePositiveInt(name string, value *int) error {
	if value != nil && *value <= 0 {
		return fmt.Errorf("%s must be > 0 when explicitly set", name)
	}
	return nil
}

func requirePositiveInt64(name string, value *int64) error {
	if value != nil && *value <= 0 {
		return fmt.Errorf("%s must be > 0 when explicitly set", name)
	}
	return nil
}

func requireNonNegativeInt64(name string, value *int64) error {
	if value != nil && *value < 0 {
		return fmt.Errorf("%s must be >= 0 when explicitly set", name)
	}
	return nil
}

func requirePositiveUint16(name string, value *uint16) error {
	if value != nil && *value == 0 {
		return fmt.Errorf("%s must be > 0 when explicitly set", name)
	}
	return nil
}
