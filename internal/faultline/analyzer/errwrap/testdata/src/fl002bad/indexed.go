package fl002bad

import "fmt"

func explicitIndex(err error) error {
	return fmt.Errorf("%[2]v", "ignored", err) // want "FL002"
}

func cursorAfterExplicit(err error) error {
	return fmt.Errorf("%[2]d %v", 0, 1, err) // want "FL002"
}

func cursorAfterIndexedPercent(err error) error {
	return fmt.Errorf("%[2]% %v", "ignored", err) // want "FL002"
}

func flagsBeforeIndex(err error) error {
	return fmt.Errorf("%+[2]v", "ignored", err) // want "FL002"
}

func indexedWidth(err error) error {
	return fmt.Errorf("%[1]*[2]v", 4, err) // want "FL002"
}

func indexedPrecision(err error) error {
	return fmt.Errorf("%.[1]*[2]v", 2, err) // want "FL002"
}

func indexedWidthAndPrecision(err error) error {
	return fmt.Errorf("%[1]*.[2]*[3]v", 4, 2, err) // want "FL002"
}

func sequentialWidthAndPrecision(err error) error {
	return fmt.Errorf("%*.*s", 4, 2, err) // want "FL002"
}

func precisionIndexSelectsValue(err error) error {
	return fmt.Errorf("%.[2]v", "ignored", err) // want "FL002"
}

func malformedIndexDoesNotConsume(err error) error {
	return fmt.Errorf("%[x]v %v", err) // want "FL002"
}

func zeroIndexDoesNotConsume(err error) error {
	return fmt.Errorf("%[0]v %s", err) // want "FL002"
}

func outOfRangeIndexDoesNotConsume(err error) error {
	return fmt.Errorf("%[2]v %v", err) // want "FL002"
}

func malformedIndexedWidthDoesNotConsumeValue(err error) error {
	return fmt.Errorf("%[1]2v %v", err) // want "FL002"
}

func malformedIndexedPrecisionDoesNotConsumeValue(err error) error {
	return fmt.Errorf("%[1].2v %v", err) // want "FL002"
}

func outOfRangeIndexedStarUsesCurrentCursor(err error) error {
	return fmt.Errorf("%[4]*v %v", 4, err) // want "FL002"
}
