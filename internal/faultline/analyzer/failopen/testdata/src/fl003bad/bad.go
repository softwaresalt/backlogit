package fl003bad

func nilError() error {
	if err := work(); err != nil { // want "FL003"
		return nil
	}
	return nil
}

func zeroValuesAndNil() (string, error) {
	if err := work(); err != nil { // want "FL003"
		return "", nil
	}
	return "ok", nil
}

func work() error {
	return nil
}
