package proxy_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thepwagner/hermit/proxy"
)

func TestNewURLData(t *testing.T) {
	res := httptest.NewRecorder()
	fmt.Fprintln(res.Body, "digestable, like the cookie")
	d := proxy.NewURLData(res)
	assert.Equal(t, "23639f4d8b6ac5a9dab675652572c3829eac86f20bfdeb77048baed05816542a", d.Sha256)
	assert.Equal(t, "b5cdac40ce88c20112ad392df6b1b5a91258b848bb69695e4059bb659adb078eeaed1d0f8d28445f33301c0b2d4052ef0c89195c340486ecfe914e93e111ecb7", d.Shake256)
}
