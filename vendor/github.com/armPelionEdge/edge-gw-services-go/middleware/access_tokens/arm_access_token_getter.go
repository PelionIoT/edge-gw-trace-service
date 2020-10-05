package access_tokens

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const authorizationHeaderName string = "Authorization"
const tokenPrefix string = "Bearer "
var accessTokenRegex *regexp.Regexp

func init() {
	var err error

	accessTokenRegex, err = regexp.Compile(fmt.Sprintf("^%s.+$", tokenPrefix))

	if err != nil {
		panic(err)
	}
}

type ArmAccessTokenGetterImpl struct {
}

func (accessTokenGetter *ArmAccessTokenGetterImpl) GetAccessToken(r *http.Request) string {
	if !accessTokenRegex.MatchString(r.Header.Get(authorizationHeaderName)) {
		return ""
	}

	return strings.TrimSpace(r.Header.Get(authorizationHeaderName)[len(tokenPrefix):])
}
