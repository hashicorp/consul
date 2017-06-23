package s3

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
)

type xmlErrorResponse struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

func unmarshalError(r *request.Request) {
	defer r.HTTPResponse.Body.Close()
	defer io.Copy(ioutil.Discard, r.HTTPResponse.Body)

	// Bucket exists in a different region, and request needs
	// to be made to the correct region.
	if r.HTTPResponse.StatusCode == http.StatusMovedPermanently {
		r.Error = awserr.NewRequestFailure(
			awserr.New("BucketRegionError",
				fmt.Sprintf("incorrect region, the bucket is not in '%s' region",
					aws.StringValue(r.Config.Region)),
				nil),
			r.HTTPResponse.StatusCode,
			r.RequestID,
		)
		return
	}

	var errCode, errMsg string

	// Attempt to parse error from body if it is known
	resp := &xmlErrorResponse{}
	err := xml.NewDecoder(r.HTTPResponse.Body).Decode(resp)
	if err != nil && err != io.EOF {
		errCode = "SerializationError"
		errMsg = "failed to decode S3 XML error response"
	} else {
		errCode = resp.Code
		errMsg = resp.Message
	}

	// Fallback to status code converted to message if still no error code
	if len(errCode) == 0 {
		statusText := http.StatusText(r.HTTPResponse.StatusCode)
		errCode = strings.Replace(statusText, " ", "", -1)
		errMsg = statusText
	}

	r.Error = awserr.NewRequestFailure(
		awserr.New(errCode, errMsg, nil),
		r.HTTPResponse.StatusCode,
		r.RequestID,
	)
}
