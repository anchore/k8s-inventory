/*
If we are providing explicit kubernetes client configuration,
this enum will expose the modes of user auth
*/
package config

import "strings"

const (
	PrivateKey UserConf = iota
	ServiceAccountToken
)

var userConfStr = []string{
	"private_key",
	"token",
}

var UserConfs = []UserConf{
	PrivateKey,
	ServiceAccountToken,
}

type UserConf int

// Parse the Mode from the user specified string (should match one of userConfStr - see above). If no matches, we fallback to adhoc
func ParseUserConf(userStr string) UserConf {
	switch strings.ToLower(userStr) {
	case strings.ToLower(ServiceAccountToken.String()):
		return ServiceAccountToken
	default:
		return PrivateKey
	}
}

// Convert the mode object to a string
func (o UserConf) String() string {
	if int(o) >= len(userConfStr) || o < 0 {
		return userConfStr[0]
	}

	return userConfStr[o]
}
