package util

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTruncateSquidError(t *testing.T) {
	in := errors.New(exampleError)
	got := tidySquidError(in)
	require.NotEqual(t, in, got)

	expect := `<div id="titles">
<h1>ERROR</h1>
<h2>The requested URL could not be retrieved</h2>
</div>
<hr>

<div id="content">
<p>The following error was encountered while trying to retrieve the URL: <a href="http://10.57.59.17:5000/">http://10.57.59.17:5000/</a></p>

<blockquote id="error">
<p><b>Connection to 10.57.59.17 failed.</b></p>
</blockquote>

<p id="sysmsg">The system returned: <i>(111) Connection refused</i></p>

<p>The remote host or network may be down. Please try the request again.</p>

<p>Your cache administrator is <a href="mailto:webmaster?subject=CacheErrorInfo%20-%20ERR_CONNECT_FAIL&amp;body=CacheHost%3A%209989a417e391%0D%0AErrPage%3A%20ERR_CONNECT_FAIL%0D%0AErr%3A%20(111)%20Connection%20refused%0D%0ATimeStamp%3A%20Tue,%2025%20Apr%202023%2016%3A47%3A47%20GMT%0D%0A%0D%0AClientIP%3A%2010.57.59.1%0D%0AServerIP%3A%2010.57.59.17%0D%0A%0D%0AHTTP%20Request%3A%0D%0APOST%20%2F%20HTTP%2F1.1%0AUser-Agent%3A%20Go-http-client%2F1.1%0D%0AContent-Length%3A%205%0D%0AContent-Type%3A%20text%2Fplain%0D%0AAccept-Encoding%3A%20gzip%0D%0AHost%3A%2010.57.59.17%3A5000%0D%0A%0D%0A%0D%0A">webmaster</a>.</p>

<br>
</div>

<hr>`

	require.Equal(t, expect, got.Error())
}

const exampleError = `
Stylesheet for Squid Error pages

ALL SORTS
OF
STUFF
AT THE TOP
<body id=ERR_CONNECT_FAIL>
        <div id="titles">
        <h1>ERROR</h1>
        <h2>The requested URL could not be retrieved</h2>
        </div>
        <hr>

        <div id="content">
        <p>The following error was encountered while trying to retrieve the URL: <a href="http://10.57.59.17:5000/">http://10.57.59.17:5000/</a></p>

        <blockquote id="error">
        <p><b>Connection to 10.57.59.17 failed.</b></p>
        </blockquote>

        <p id="sysmsg">The system returned: <i>(111) Connection refused</i></p>

        <p>The remote host or network may be down. Please try the request again.</p>

        <p>Your cache administrator is <a href="mailto:webmaster?subject=CacheErrorInfo%20-%20ERR_CONNECT_FAIL&amp;body=CacheHost%3A%209989a417e391%0D%0AErrPage%3A%20ERR_CONNECT_FAIL%0D%0AErr%3A%20(111)%20Connection%20refused%0D%0ATimeStamp%3A%20Tue,%2025%20Apr%202023%2016%3A47%3A47%20GMT%0D%0A%0D%0AClientIP%3A%2010.57.59.1%0D%0AServerIP%3A%2010.57.59.17%0D%0A%0D%0AHTTP%20Request%3A%0D%0APOST%20%2F%20HTTP%2F1.1%0AUser-Agent%3A%20Go-http-client%2F1.1%0D%0AContent-Length%3A%205%0D%0AContent-Type%3A%20text%2Fplain%0D%0AAccept-Encoding%3A%20gzip%0D%0AHost%3A%2010.57.59.17%3A5000%0D%0A%0D%0A%0D%0A">webmaster</a>.</p>

        <br>
        </div>

        <hr>
        <div id="footer">
MORE STUFF AT THE BOTTOM
YEP
`
