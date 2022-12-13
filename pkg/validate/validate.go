package validate

import "fmt"

func ValidateList(l []string) error {
	if len(l) == 0 {
		return nil
	}

	for _, i := range l {
		if _, ok := countryCodes[i]; !ok {
			return fmt.Errorf("%s not found in countryCodes map, check https://www.ripe.net/participate/member-support/list-of-members/list-of-country-codes-and-rirs", i)
		}
	}
	return nil
}
